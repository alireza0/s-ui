package nodes

import (
	"compress/gzip"
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"regexp"
	"time"

	"github.com/alireza0/s-ui/database"
	"github.com/alireza0/s-ui/database/model"
	"github.com/alireza0/s-ui/nodeagent"
	"golang.org/x/crypto/ssh"
	"gorm.io/gorm"
)

type provisionListener struct {
	Key     string `json:"key"`
	Network string `json:"network"`
	Port    int    `json:"port"`
}

// Resolve only listener ports for preflight. It neither creates NodeInbounds
// nor pretends an uninstalled certificate is ready.
func plannedLiveListeners(tx *gorm.DB, node model.Node, host string, occupied map[listenerSocket]bool) ([]provisionListener, error) {
	requests, err := loadListenerRequests(tx, node)
	if err != nil {
		return nil, err
	}
	return reserveListenerPlan(tx, node.Id, host, requests, occupied)
}

func stageLiveJob(job *model.NodeJob, plan liveProvisionPlan, stage string) error {
	if err := saveLivePlan(job, plan); err != nil {
		return err
	}
	jobStage(job, stage, "running", "")
	// Do not perform a remote mutation unless its intent is durable.
	var saved model.NodeJob
	if err := database.GetDB().First(&saved, "id = ?", job.Id).Error; err != nil {
		return err
	}
	if saved.Stage != stage || saved.Plan != job.Plan || saved.State != "running" {
		return errors.New("install_checkpoint_failed")
	}
	return nil
}

func liveControlValid(node model.Node, installation model.NodeInstallation) bool {
	return controlIdentityValid(node, installation, time.Now())
}

func executeLiveProvision(ctx context.Context, job *model.NodeJob, plan *liveProvisionPlan, password string) error {
	db := database.GetDB()
	var node model.Node
	if err := db.First(&node, job.NodeId).Error; err != nil {
		return err
	}
	if node.ManagementGeneration != plan.ExpectedGeneration {
		return errors.New("management_target_changed")
	}
	if err := stageLiveJob(job, *plan, "ssh"); err != nil {
		return err
	}
	client, err := connectSSH(ctx, &plan.Target, password, plan.Request.ReplaceHostKey && password != "")
	password = ""
	if err != nil {
		return err
	}
	defer client.Close()
	if !plan.Previous.Ready {
		if err := db.Model(&model.NodeInstallation{}).Where("node_id = ? AND owner_id = ? AND ip = ? AND ssh_port = ? AND ssh_user = ? AND ready = false", job.NodeId, plan.Target.OwnerID, plan.Target.IP, plan.Target.SSHPort, plan.Target.SSHUser).Update("host_key", plan.Target.HostKey).Error; err != nil {
			return err
		}
	}
	if plan.Committed {
		status, err := readInstalledIdentity(ctx, client, *plan)
		if err != nil {
			return err
		}
		if status.Epoch != node.ControlEpoch {
			return errors.New("agent_epoch_changed_recovery_required")
		}
		// A panel restart resumes verification, not a second installation that
		// could momentarily start a cached ACL after an acknowledged revoke.
		return verifyLiveProvision(ctx, job, plan)
	}
	if err = stageLiveJob(job, *plan, "preflight"); err != nil {
		return err
	}
	host, err := inspectProvisionHost(ctx, client, plan.Target.SSHUser)
	if err != nil {
		return err
	}
	artifact, digest, err := agentArtifact(host.architecture)
	if err != nil {
		return errors.New("agent_build_unavailable")
	}
	if err = stageLiveJob(job, *plan, "dependencies"); err != nil {
		return err
	}
	if err = ensureRemotePython(ctx, client, plan.Target.SSHUser, host); err != nil {
		return err
	}
	if err = stageLiveJob(job, *plan, "inspect"); err != nil {
		return err
	}
	requests, err := loadListenerRequests(db, node)
	if err != nil {
		return err
	}
	inspection, occupied, err := inspectListenerSockets(ctx, client, plan.Target, requests)
	if err != nil {
		return err
	}
	if inspection.Arch != host.architecture {
		return errors.New("unsupported_architecture")
	}
	err = db.Transaction(func(tx *gorm.DB) error {
		var current model.Node
		if err := tx.First(&current, node.Id).Error; err != nil {
			return err
		}
		if !sameListenerDraft(node, current) {
			return errors.New("management_target_changed")
		}
		var e error
		plan.Listeners, e = plannedLiveListeners(tx, current, plan.Target.IP, occupied)
		return e
	})
	if err != nil {
		return err
	}
	plan.Target.ControlPort = inspection.ControlPort
	plan.Target.Architecture = host.architecture
	if err := prepareLiveControlIdentity(db, job.NodeId, node, plan); err != nil {
		return err
	}
	key, err := openSecret(plan.Target.PrivateKey)
	if err != nil {
		return err
	}
	authorized, err := parseProvisionSSHKey(key)
	if err != nil {
		return err
	}
	if err = stageLiveJob(job, *plan, "ssh_key"); err != nil {
		return err
	}
	if _, err = remotePython(ctx, client, plan.Target.SSHUser, liveAuthorizeKeyScript, map[string]interface{}{"id": job.NodeId, "owner": plan.Target.OwnerID, "ssh_user": plan.Target.SSHUser, "control_port": plan.Target.ControlPort, "authorized_key": authorized}); err != nil {
		return err
	}
	keyTarget := plan.Target
	keyClient, err := connectSSH(ctx, &keyTarget, "", false)
	if err != nil {
		return errors.New("ssh_key_verification_failed")
	}
	keyClient.Close()
	if err = stageLiveJob(job, *plan, "upload"); err != nil {
		return err
	}
	if err = uploadLiveAgent(ctx, client, plan.Target, artifact, digest); err != nil {
		return err
	}
	if err := fenceLiveInstallation(ctx, client, job, plan); err != nil {
		return err
	}
	if err = stageLiveJob(job, *plan, "install"); err != nil {
		return err
	}
	cert, err := openSecret(plan.Target.ServerCertificate)
	if err != nil {
		return err
	}
	serverKey, err := openSecret(plan.Target.ServerKey)
	if err != nil {
		return err
	}
	input := map[string]interface{}{"id": job.NodeId, "owner": plan.Target.OwnerID, "bootstrap_id": plan.Target.BootstrapID, "ssh_user": plan.Target.SSHUser, "control_port": plan.Target.ControlPort, "sha256": digest, "ca": plan.Control.CA, "certificate": cert, "key": serverKey, "authorized_key": authorized, "listeners": plan.Listeners, "generation": plan.ExpectedGeneration + 1}
	if err = installRemoteAgent(ctx, client, plan.Target.SSHUser, liveInstallScript, input); err != nil {
		return err
	}
	if err = stageLiveJob(job, *plan, "register"); err != nil {
		return err
	}
	status, err := readInstalledIdentity(ctx, client, *plan)
	if err != nil {
		return err
	}
	if status.SingBoxVersion != nodeagent.SingBoxVersion() || status.AgentDigest != digest {
		return errors.New("agent_build_mismatch")
	}
	if !plan.Committed {
		if err = commitLiveTarget(job, plan, status); err != nil {
			return err
		}
	}
	return verifyLiveProvision(ctx, job, plan)
}

func verifyLiveProvision(ctx context.Context, job *model.NodeJob, plan *liveProvisionPlan) error {
	if err := stageLiveJob(job, *plan, "verify"); err != nil {
		return err
	}
	for ctx.Err() == nil {
		// Import/restart may replace the pool while this durable job is waiting.
		db := database.GetDB()
		_ = ReconcileAccess(ctx, job.NodeId)
		if err := pollLiveProvision(ctx, db, job.NodeId, job); err != nil {
			return err
		}
		if liveProvisionReady(job.NodeId) {
			return markLiveProvisionReady(db, job.NodeId, plan)
		}
		var structureTask model.NodeTask
		if db.First(&structureTask, "node_id = ? AND kind = ?", job.NodeId, "structure").Error == nil && structureTask.State == "rejected" {
			if structureTask.LastError == errRealityHandshakeLoop.Error() {
				return errRealityHandshakeLoop
			}
			return errors.New("structural_revision_rejected")
		}
		select {
		case <-ctx.Done():
		case <-time.After(2 * time.Second):
		}
	}
	return errors.New("install_timeout")
}

func readInstalledIdentity(ctx context.Context, client *ssh.Client, plan liveProvisionPlan) (nodeagent.StatusV2, error) {
	var status nodeagent.StatusV2
	identity, err := tls.X509KeyPair([]byte(plan.Control.Certificate), []byte(plan.Control.Key))
	if err != nil {
		return status, err
	}
	roots := x509.NewCertPool()
	if !roots.AppendCertsFromPEM([]byte(plan.Control.CA)) {
		return status, errors.New("invalid_control_ca")
	}
	transport := &http.Transport{TLSClientConfig: &tls.Config{MinVersion: tls.VersionTLS13, RootCAs: roots, Certificates: []tls.Certificate{identity}}, DialContext: client.DialContext}
	defer transport.CloseIdleConnections()
	control := &http.Client{Transport: transport, Timeout: 8 * time.Second, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
	for attempt := 0; attempt < 10; attempt++ {
		req, err := http.NewRequestWithContext(ctx, "GET", controlAddress(plan.Target.ControlPort)+"/v2/status", nil)
		if err != nil {
			return status, err
		}
		response, err := control.Do(req)
		if err == nil {
			err = json.NewDecoder(io.LimitReader(response.Body, 1<<20)).Decode(&status)
			response.Body.Close()
			if err == nil && response.StatusCode == 200 {
				if status.NodeID != uint64(plan.Target.NodeId) || status.Protocol != 2 || status.Epoch == "" || status.BootstrapID != plan.Target.BootstrapID {
					return status, errors.New("agent_bootstrap_identity_mismatch")
				}
				return status, nil
			}
		}
		select {
		case <-ctx.Done():
			return status, ctx.Err()
		case <-time.After(time.Second):
		}
	}
	return status, errors.New("agent_startup_unreachable")
}

func commitLiveTarget(job *model.NodeJob, plan *liveProvisionPlan, status nodeagent.StatusV2) error {
	db := database.GetDB()
	committed := *plan
	committed.ExpectedGeneration++
	committed.Committed = true
	sealed, err := encodeLivePlan(committed)
	if err != nil {
		return err
	}
	err = db.Transaction(func(tx *gorm.DB) error {
		var current model.Node
		if err := tx.First(&current, job.NodeId).Error; err != nil {
			return err
		}
		if current.ManagementGeneration != plan.ExpectedGeneration {
			return errors.New("management_target_changed")
		}
		if !plan.Fenced || status.NodeID != uint64(current.Id) || status.BootstrapID != plan.Target.BootstrapID {
			return errors.New("agent_bootstrap_identity_mismatch")
		}
		if err := tx.Save(&plan.Target).Error; err != nil {
			return err
		}
		// Counters and client material are retained. New epoch is explicitly accepted
		// only here after pinned SSH, mTLS and this installation's bootstrap receipt.
		updates := map[string]interface{}{"name": plan.Request.Name, "address": controlAddress(plan.Target.ControlPort), "control_ca": plan.Control.CA, "control_certificate": plan.Control.Certificate, "control_key": plan.Control.Key, "control_epoch": status.Epoch, "management_generation": current.ManagementGeneration + 1, "agent_protocol": 2, "agent_running": false, "agent_observed_at": time.Now().Unix(), "last_seen": 0, "tls_status": json.RawMessage(`{}`), "desired_fingerprint": "", "last_error": "", "rejected_revision": 0}
		updates["sing_box_version"], updates["agent_digest"] = status.SingBoxVersion, status.AgentDigest
		updates["agent_extended_inbounds"] = status.ExtendedInbounds
		updates["agent_proxy_inbounds"] = status.ProxyInbounds
		updates["agent_upgrade_retry_at"], updates["agent_upgrade_state"] = 0, ""
		if err := tx.Model(&current).Updates(updates).Error; err != nil {
			return err
		}
		// A stored leaf is not proof that this registration has received it.
		if err := tx.Model(&model.NodeCertificateBinding{}).Where(whereNodeID, current.Id).Updates(map[string]interface{}{"state": "pending", "applied_revision": 0, "last_error": ""}).Error; err != nil {
			return err
		}
		var domain model.NodeDomainBinding
		if tx.First(&domain, whereNodeID, current.Id).Error == nil {
			if domain.ExpectedIP == plan.Previous.IP && domain.ExpectedIP != plan.Target.IP {
				if err := tx.Model(&domain).Updates(map[string]interface{}{"expected_ip": plan.Target.IP, "generation": domain.Generation + 1, "state": "pending", "last_error": ""}).Error; err != nil {
					return err
				}
			}
		} else if err := tx.Model(&current).Update("link_address", plan.Target.IP).Error; err != nil {
			return err
		}
		if err := queueNodeStructure(tx, &current); err != nil {
			return err
		}
		if err := database.QueueNodeTasks(tx, []uint{job.NodeId}, "access"); err != nil {
			return err
		}
		if err := database.QueueNodeTasks(tx, []uint{job.NodeId}, "resources"); err != nil {
			return err
		}
		return tx.Model(job).Update("plan", sealed).Error
	})
	if err != nil {
		return err
	}
	*plan = committed
	job.Plan = sealed
	return nil
}

func liveProvisionReady(id uint) bool {
	db := database.GetDB()
	var n model.Node
	var access model.NodeAccessState
	if db.First(&n, id).Error != nil || db.First(&access, whereNodeID, id).Error != nil {
		return false
	}
	if n.DesiredRevision == 0 || n.DesiredRevision != n.AppliedRevision || n.TemplateTargetVersion != n.AppliedTemplateVersion || access.DesiredRevision == 0 || access.DesiredRevision != access.AppliedRevision || access.LastError != "" || n.LastError != "" || (!n.AgentRunning && n.Enable) {
		return false
	}
	if !liveProvisionCertificatesReady(n.TLSStatus) {
		return false
	}
	if !n.Enable {
		return true
	}
	var domain model.NodeDomainBinding
	if db.First(&domain, whereNodeID, id).Error == nil {
		if domain.State != "ready" {
			return false
		}
		var cert model.NodeCertificateBinding
		if db.First(&cert, whereNodeID, id).Error != nil || cert.State != "installed" {
			return false
		}
	}
	return true
}

func uploadLiveAgent(ctx context.Context, client *ssh.Client, install model.NodeInstallation, path, digest string) error {
	if !regexp.MustCompile(`^[a-f0-9]{32}$`).MatchString(install.OwnerID) {
		return errors.New("invalid_installation_owner")
	}
	body, err := remotePython(ctx, client, install.SSHUser, liveCachedAgentScript, map[string]interface{}{"id": install.NodeId, "owner": install.OwnerID, "sha256": digest})
	if err != nil {
		return err
	}
	var cached struct {
		Cached bool `json:"cached"`
	}
	if json.Unmarshal(body, &cached) != nil {
		return errors.New("remote_response_invalid")
	}
	if cached.Cached {
		return nil
	}
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer f.Close()
	slot := fmt.Sprintf("%d-%s", install.NodeId, install.OwnerID[:12])
	script := fmt.Sprintf(`import gzip,hashlib,json,os,sys
from pathlib import Path
folder=Path('/opt/s-ui-nodes/%s')
if folder.is_symlink():raise SystemExit('unsafe path')
folder.parent.mkdir(mode=0o755,exist_ok=True)
folder.mkdir(mode=0o755,exist_ok=True)
fd=os.open(folder/'agent.next',os.O_WRONLY|os.O_CREAT|os.O_TRUNC|os.O_NOFOLLOW,0o600);h=hashlib.sha256()
with os.fdopen(fd,'wb') as f,gzip.GzipFile(fileobj=sys.stdin.buffer,mode='rb') as source:
 while True:
  data=source.read(1<<20)
  if not data:break
  f.write(data);h.update(data)
 f.flush();os.fsync(f.fileno())
if h.hexdigest()!='%s':raise SystemExit('checksum mismatch')
print(json.dumps({'verified':True}))`, slot, digest)
	return compressedAgentStream(f, func(input io.Reader) error {
		_, err := remoteStream(ctx, client, install.SSHUser, script, input)
		return err
	})
}

// Compression affects transport only; the remote host verifies the original
// artifact digest before it can replace the installed executable.
func compressedAgentStream(input io.Reader, send func(io.Reader) error) error {
	reader, writer := io.Pipe()
	done := make(chan error, 1)
	go func() {
		compressed, _ := gzip.NewWriterLevel(writer, gzip.BestSpeed)
		_, err := io.Copy(compressed, input)
		closeErr := compressed.Close()
		if err == nil {
			err = closeErr
		}
		_ = writer.CloseWithError(err)
		done <- err
	}()
	err := send(reader)
	_ = reader.CloseWithError(err)
	return errors.Join(err, <-done)
}

const liveHostHeader = `import json,os,platform,re,socket,subprocess,sys
from pathlib import Path
p=json.load(sys.stdin);ident=int(p['id']);owner=p['owner']
assert ident>0 and re.fullmatch('[a-f0-9]{32}',owner)
slot=str(ident)+'-'+owner[:12];name='s-ui-node-'+slot
unit=Path('/etc/systemd/system')/(name+'.service');marker='# S-UI managed node '+str(ident)+' owner='+owner
if os.geteuid()!=0 or not Path('/run/systemd/system').exists():raise SystemExit('root/systemd required')
if unit.is_symlink():raise SystemExit('unsafe unit path')
if unit.exists() and marker not in unit.read_text():raise SystemExit('foreign service')
`

var liveAuthorizeKeyScript = liveHostHeader + `
import pwd,tempfile,fcntl
account=pwd.getpwnam(p['ssh_user']);folder=Path(account.pw_dir)/'.ssh';target=folder/'authorized_keys'
if folder.is_symlink() or target.is_symlink():raise SystemExit('unsafe SSH key path')
folder.mkdir(mode=0o700,exist_ok=True);os.chmod(folder,0o700);os.chown(folder,account.pw_uid,account.pw_gid)
keylock=os.open(folder/'.s-ui-node-key.lock',os.O_RDWR|os.O_CREAT|os.O_NOFOLLOW,0o600);fcntl.flock(keylock,fcntl.LOCK_EX)
prior=target.read_text() if target.exists() else ''
keymarker='s-ui-managed-node-'+slot;port=int(p['control_port']);assert 1024<=port<=65535
key=p['authorized_key'].strip();assert '\n' not in key and '\r' not in key
lines=[line for line in prior.splitlines() if not line.endswith(' '+keymarker)]
lines.append('restrict,port-forwarding,permitopen="127.0.0.1:'+str(port)+'" '+key+' '+keymarker)
fd,name=tempfile.mkstemp(prefix='.s-ui-key-',dir=folder)
try:
 with os.fdopen(fd,'w') as output:output.write('\n'.join(lines)+'\n');output.flush();os.fsync(output.fileno())
 os.chmod(name,0o600);os.chown(name,account.pw_uid,account.pw_gid);os.replace(name,target)
finally:
 if os.path.exists(name):os.unlink(name)
print(json.dumps({'authorized':True}))
`

var liveCachedAgentScript = liveHostHeader + `
import hashlib,shutil,tempfile
folder=Path('/opt/s-ui-nodes')/slot;binary=folder/'agent'
if folder.is_symlink() or binary.is_symlink():raise SystemExit('unsafe artifact path')
cached=binary.is_file() and hashlib.sha256(binary.read_bytes()).hexdigest()==p['sha256']
if cached:
 fd,temporary=tempfile.mkstemp(prefix='.agent-cache-',dir=folder);os.close(fd)
 try:
  shutil.copyfile(binary,temporary)
  if hashlib.sha256(Path(temporary).read_bytes()).hexdigest()!=p['sha256']:raise SystemExit('artifact changed')
  os.replace(temporary,folder/'agent.next')
 finally:
  if os.path.exists(temporary):os.unlink(temporary)
print(json.dumps({'cached':cached}))
`

var liveFenceScript = provisionFirewallPython + liveHostHeader + `
if unit.exists():
 subprocess.run(['systemctl','disable','--now',name],check=True,capture_output=True)
 if subprocess.run(['systemctl','is-active','--quiet',name]).returncode==0:raise SystemExit('unit still active')
binary='/opt/s-ui-nodes/'+slot+'/agent'
for proc in Path('/proc').glob('[0-9]*/cmdline'):
 try:parts=proc.read_bytes().split(b'\0')
 except (OSError,PermissionError):continue
 if binary.encode() in parts:raise SystemExit('owned process still active')
release_node_firewall(name)
print(json.dumps({'fenced':True}))
`

var liveInspectionScript = liveHostHeader + `
pid=0
if unit.exists():
 r=subprocess.run(['systemctl','show','-p','MainPID','--value',name],capture_output=True,text=True)
 if r.returncode==0:pid=int(r.stdout.strip() or '0')
owned=set()
if pid>0:
 for fd in Path('/proc/'+str(pid)+'/fd').glob('*'):
  try:target=os.readlink(fd)
  except OSError:continue
  if target.startswith('socket:['):owned.add(target[8:-1])
flows={}
for network in ('tcp','udp'):
 for suffix in ('','6'):
  path=Path('/proc/net/'+network+suffix)
  if not path.exists():continue
  for line in path.read_text().splitlines()[1:]:
   row=line.split()
   if network=='udp' or row[3]=='0A':
    port=int(row[1].rsplit(':',1)[1],16)
    flows.setdefault((network,port),set()).add(row[9])
def free(port,network,control=False):
 inodes=flows.get((network,port),set())
 if inodes:return inodes.issubset(owned)
 kind=socket.SOCK_DGRAM if network=='udp' else socket.SOCK_STREAM
 for family,address in [(socket.AF_INET,'127.0.0.1' if control else '0.0.0.0'),(socket.AF_INET6,'::1' if control else '::')]:
  try:
   with socket.socket(family,kind) as sock:
    if network=='tcp':sock.setsockopt(socket.SOL_SOCKET,socket.SO_REUSEADDR,1)
    if family==socket.AF_INET6:sock.setsockopt(socket.IPPROTO_IPV6,socket.IPV6_V6ONLY,1)
    sock.bind((address,port))
  except OSError as e:
   if family==socket.AF_INET6 and e.errno in (97,99):continue
   return False
 return True
occupied=[]
for listener in p['probes']:
 if listener['network'] not in ('tcp','udp') or not 1<=int(listener['port'])<=65535:raise SystemExit('invalid listener')
 if not free(int(listener['port']),listener['network']):occupied.append(listener)
control=int(p.get('control_port') or 0)
if control and not free(control,'tcp',True):print(json.dumps({'error':'node_listener_port_occupied'}));raise SystemExit(0)
if not control:
 fixed=set(p.get('fixed_tcp_ports') or [])
 control=next((port for port in range(16500,16600) if port not in fixed and free(port,'tcp',True)),0)
arch={'x86_64':'amd64','aarch64':'arm64'}.get(platform.machine(),'unsupported')
print(json.dumps({'arch':arch,'control_port':control,'occupied':occupied}))
`

func prepareLiveControlIdentity(db *gorm.DB, nodeID uint, node model.Node, plan *liveProvisionPlan) error {
	if plan.Target.PrivateKey == "" {
		private, _, err := newSSHKey()
		if err != nil {
			return err
		}
		plan.Target.PrivateKey, err = sealSecret(private)
		if err != nil {
			return err
		}
	}
	candidate := node
	candidate.ControlCA, candidate.ControlCertificate, candidate.ControlKey = plan.Control.CA, plan.Control.Certificate, plan.Control.Key
	if !liveControlValid(candidate, plan.Target) {
		if err := newControlIdentity(&candidate, &plan.Target); err != nil {
			return err
		}
		plan.Control = ControlTLS{candidate.ControlCA, candidate.ControlCertificate, candidate.ControlKey}
	}
	if !plan.Previous.Ready {
		if err := db.Model(&model.NodeInstallation{}).Where("node_id = ? AND owner_id = ? AND ip = ? AND ssh_port = ? AND ssh_user = ? AND ready = false", nodeID, plan.Target.OwnerID, plan.Target.IP, plan.Target.SSHPort, plan.Target.SSHUser).Update("private_key", plan.Target.PrivateKey).Error; err != nil {
			return err
		}
	}
	return nil
}

func pollLiveProvision(ctx context.Context, db *gorm.DB, id uint, job *model.NodeJob) error {
	var domain model.NodeDomainBinding
	domainErr := db.First(&domain, whereNodeID, id).Error
	if domainErr != nil && !errors.Is(domainErr, gorm.ErrRecordNotFound) {
		return errors.New("installation_database_unavailable")
	}
	if domainErr == nil {
		jobStage(job, "certificate", "running", "")
		if resourceErr := ReconcileResources(ctx, id); errors.Is(resourceErr, errV2Rejected) {
			return errors.New("certificate_request_rejected")
		}
	}
	structureErr := Reconcile(ctx, id)
	if errors.Is(structureErr, errListenerOccupied) || errors.Is(structureErr, errListenerReserved) || errors.Is(structureErr, errAutoPortsExhausted) || errors.Is(structureErr, errRealityHandshakeLoop) {
		return structureErr
	}
	if errors.Is(structureErr, errV2Rejected) || (structureErr != nil && structureErr.Error() == "structural_revision_rejected") {
		return errors.New("structural_revision_rejected")
	}
	return nil
}

func markLiveProvisionReady(db *gorm.DB, id uint, plan *liveProvisionPlan) error {
	return db.Transaction(func(tx *gorm.DB) error {
		var current model.Node
		if e := tx.First(&current, id).Error; e != nil {
			return e
		}
		if current.ManagementGeneration != plan.ExpectedGeneration {
			return errors.New("management_target_changed")
		}
		if e := tx.Model(&model.NodeInstallation{}).Where(whereNodeID, id).Update("ready", true).Error; e != nil {
			return e
		}
		return tx.Where("node_id = ? AND host <> ?", id, plan.Target.IP).Delete(&model.NodePortReservation{}).Error
	})
}

func liveProvisionCertificatesReady(raw json.RawMessage) bool {
	var report struct {
		Certificates []struct {
			State string `json:"state"`
		} `json:"certificates"`
	}
	if json.Unmarshal(raw, &report) != nil {
		return false
	}
	for _, c := range report.Certificates {
		if c.State != "ready" && c.State != "not_required" {
			return false
		}
	}
	return true
}

func fenceLiveInstallation(ctx context.Context, client *ssh.Client, job *model.NodeJob, plan *liveProvisionPlan) error {
	if !plan.Fenced {
		if err := stageLiveJob(job, *plan, "retire"); err != nil {
			return err
		}
		if plan.Previous.IP != "" && (plan.Previous.IP != plan.Target.IP || plan.Previous.SSHPort != plan.Target.SSHPort || plan.Previous.SSHUser != plan.Target.SSHUser) {
			old := plan.Previous
			oldClient, e := connectSSH(ctx, &old, "", false)
			if e != nil {
				return errors.New("old_server_retirement_pending")
			}
			_, e = remotePython(ctx, oldClient, old.SSHUser, liveFenceScript, map[string]interface{}{"id": job.NodeId, "owner": old.OwnerID})
			oldClient.Close()
			if e != nil {
				return errors.New("old_server_retirement_pending")
			}
		}
		// A previous failed attempt may already have a unit on this candidate.
		if _, err := remotePython(ctx, client, plan.Target.SSHUser, liveFenceScript, map[string]interface{}{"id": job.NodeId, "owner": plan.Target.OwnerID}); err != nil {
			return err
		}
		plan.Fenced = true
	}
	return nil
}
