package nodes

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"golang.org/x/crypto/ssh"
)

func agentArtifact(arch string) (string, string, error) {
	if arch != "amd64" && arch != "arm64" {
		return "", "", errors.New("unsupported_architecture")
	}
	folder, err := agentArtifactsDirectory()
	if err != nil {
		return "", "", err
	}
	file := filepath.Join(folder, "s-ui-node-linux-"+arch)
	digest, err := verifyAgentArtifact(file)
	if err != nil {
		return "", "", err
	}
	bundle, err := readAgentBundle()
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return "", "", err
	}
	if err == nil && bundle.Agents[arch] != digest {
		return "", "", errors.New("agent bundle checksum mismatch")
	}
	return file, digest, nil
}

func verifyAgentArtifact(file string) (string, error) {
	f, err := os.Open(file)
	if err != nil {
		return "", err
	}
	defer f.Close()
	stat, err := f.Stat()
	if err != nil || !stat.Mode().IsRegular() {
		return "", errors.New("invalid agent artifact")
	}
	h := sha256.New()
	if _, err = io.Copy(h, f); err != nil {
		return "", err
	}
	digest := hex.EncodeToString(h.Sum(nil))
	manifest, err := os.ReadFile(file + ".sha256")
	if err != nil || len(strings.Fields(string(manifest))) == 0 || strings.Fields(string(manifest))[0] != digest {
		return "", errors.New("agent checksum mismatch")
	}
	return digest, nil
}

func agentArtifactsDirectory() (string, error) {
	if dir := os.Getenv("SUI_NODE_AGENT_DIR"); dir != "" {
		return dir, nil
	}
	executable, err := os.Executable()
	if err != nil {
		return "", err
	}
	return filepath.Join(filepath.Dir(executable), "agents"), nil
}

func parseProvisionSSHKey(key string) (string, error) {
	s, err := ssh.ParsePrivateKey([]byte(key))
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(ssh.MarshalAuthorizedKey(s.PublicKey()))), nil
}

func installRemoteAgent(parent context.Context, client *ssh.Client, user, script string, input interface{}) error {
	ctx, cancel := context.WithTimeout(parent, 4*time.Minute)
	defer cancel()
	stop := context.AfterFunc(ctx, func() { _ = client.Close() })
	defer stop()
	_, err := remotePython(ctx, client, user, script, input)
	if ctx.Err() == context.DeadlineExceeded {
		return errors.New("node_service_install_timeout")
	}
	if err != nil {
		return errors.New("node_service_install_failed")
	}
	return nil
}

var liveInstallScript = `import fcntl,hashlib,json,os,pwd,re,shutil,subprocess,sys
from pathlib import Path
` + provisionFirewallPython + `
p=json.load(sys.stdin);ident=int(p['id']);owner=p['owner'];assert re.fullmatch('[a-f0-9]{32}',owner);slot=str(ident)+'-'+owner[:12];name='s-ui-node-'+slot
assert re.fullmatch('[a-zA-Z0-9_-]{16,96}',p['bootstrap_id'])
assert ident>0 and 1024<=int(p['control_port'])<=65535
base=Path('/opt/s-ui-nodes')/slot;creds=Path('/etc/s-ui-nodes')/slot
unit=Path('/etc/systemd/system')/(name+'.service');marker='# S-UI managed node '+str(ident)+' owner='+owner
if unit.exists() and marker not in unit.read_text():raise SystemExit('service name occupied')
if creds.is_symlink() or base.is_symlink():raise SystemExit('unsafe path')
creds.mkdir(parents=True,mode=0o700,exist_ok=True);os.chmod(creds,0o700)
incoming=base/'agent.next';binary=base/'agent'
if hashlib.sha256(incoming.read_bytes()).hexdigest()!=p['sha256']:raise SystemExit('checksum mismatch')
account=pwd.getpwnam(p['ssh_user']);sshdir=Path(account.pw_dir)/'.ssh'
sshdir.mkdir(mode=0o700,exist_ok=True);authorized=sshdir/'authorized_keys'
keylock=os.open(sshdir/'.s-ui-node-key.lock',os.O_RDWR|os.O_CREAT|os.O_NOFOLLOW,0o600);fcntl.flock(keylock,fcntl.LOCK_EX)
prior=authorized.read_text() if authorized.exists() else ''
key=p['authorized_key'].strip();keymarker='s-ui-managed-node-'+slot
lines=[line for line in prior.splitlines() if not line.endswith(' '+keymarker)]
lines.append('restrict,port-forwarding,permitopen="127.0.0.1:'+str(p['control_port'])+'" '+key+' '+keymarker)
authorized.write_text('\n'.join(lines)+'\n');os.chmod(authorized,0o600);os.chown(sshdir,account.pw_uid,account.pw_gid);os.chown(authorized,account.pw_uid,account.pw_gid)
oldunit=unit.read_bytes() if unit.exists() else None
oldcreds={n:(creds/n).read_bytes() for n in ['ca.pem','cert.pem','key.pem'] if (creds/n).exists()}
oldbinary=binary.exists()
if oldbinary:shutil.copy2(binary,base/'agent.previous')
def write_private(path,body):
 tmp=path.with_suffix(path.suffix+'.new')
 fd=os.open(tmp,os.O_WRONLY|os.O_CREAT|os.O_TRUNC|os.O_NOFOLLOW,0o600)
 with os.fdopen(fd,'wb') as file:file.write(body);file.flush();os.fsync(file.fileno())
 os.replace(tmp,path);os.chmod(path,0o600)
def systemctl(*args,wait=30,check=True):
 return subprocess.run(['systemctl',*args],check=check,capture_output=True,timeout=wait)
try:
 for n,keyname in [('ca.pem','ca'),('cert.pem','certificate'),('key.pem','key')]:write_private(creds/n,p[keyname].encode())
 os.chmod(incoming,0o755);os.replace(incoming,binary)
 text=marker+'''
[Unit]
Description=S-UI managed VPN node
After=network-online.target
Wants=network-online.target
[Service]
Type=simple
DynamicUser=yes
StateDirectory='''+name+'''
StateDirectoryMode=0700
WorkingDirectory=/var/lib/'''+name+'''
LoadCredential=ca.pem:'''+str(creds/'ca.pem')+'''
LoadCredential=cert.pem:'''+str(creds/'cert.pem')+'''
LoadCredential=key.pem:'''+str(creds/'key.pem')+'''
ExecStart='''+str(binary)+''' --node-id '''+str(ident)+''' --state /var/lib/'''+name+''' --listen 127.0.0.1:'''+str(p['control_port'])+''' --bootstrap-id '''+p['bootstrap_id']+''' --client-ca %d/ca.pem --tls-cert %d/cert.pem --tls-key %d/key.pem
Restart=on-failure
RestartSec=3
TimeoutStartSec=60
TimeoutStopSec=60
NoNewPrivileges=yes
ProtectSystem=strict
ProtectHome=yes
PrivateTmp=yes
AmbientCapabilities=CAP_NET_BIND_SERVICE
CapabilityBoundingSet=CAP_NET_BIND_SERVICE
RestrictAddressFamilies=AF_INET AF_INET6 AF_UNIX AF_NETLINK
UMask=0077
LimitNOFILE=65536
[Install]
WantedBy=multi-user.target
'''
 unit.write_text(text);os.chmod(unit,0o644)
 systemctl('daemon-reload')
 systemctl('enable',name)
 systemctl('restart',name,wait=90)
 systemctl('is-active','--quiet',name)
 configure_node_firewall(p['listeners'],name,version={'generation':p.get('generation',0),'operation':p.get('bootstrap_id','')},initialize=True)
 configure_node_firewall([],name+'-http01',confirmed=True,version={'generation':p.get('generation',0),'operation':p.get('bootstrap_id','')},initialize=True)
 print(json.dumps({'installed':True,'service':name}))
except BaseException:
 if oldbinary:shutil.copy2(base/'agent.previous',binary)
 for n,body in oldcreds.items():write_private(creds/n,body)
 if oldunit is not None:
  unit.write_bytes(oldunit);systemctl('daemon-reload',check=False)
  # A failed installation stays fenced.
  systemctl('stop',name,wait=90,check=False)
 raise
`
