package nodes

import (
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"errors"
	"net"
	"net/url"
	"strings"

	"github.com/alireza0/s-ui/database"
	"github.com/alireza0/s-ui/database/model"
	"golang.org/x/net/idna"
	"gorm.io/gorm"
)

type ControlTLS struct {
	CA          string `json:"ca"`
	Certificate string `json:"certificate"`
	Key         string `json:"key"`
}

func IsObject(name string) bool {
	return name == "nodes" || name == "nodetemplates"
}

func Bundle() (map[string]interface{}, error) {
	db := database.GetDB()
	nodes := []model.Node{}
	templates := []model.NodeTemplate{}
	inbounds := []model.NodeInbound{}
	if err := db.Order("id").Find(&nodes).Error; err != nil {
		return nil, err
	}
	var members []model.NodeTemplateMember
	if err := db.Order("position").Find(&members).Error; err != nil {
		return nil, err
	}
	selections := map[uint][]uint{}
	for _, member := range members {
		selections[member.TemplateId] = append(selections[member.TemplateId], member.InboundId)
	}
	for i := range nodes {
		nodes[i].InboundTemplateIds = selections[nodes[i].TemplateId]
		nodes[i].AgentTrafficAvailable = nodes[i].TrafficProcess != ""
		nodes[i].DisableConfirmed = disableConfirmed(nodes[i])
		if !managedNodeModel(nodes[i]) {
			nodes[i].LastError = "unsupported_node_model"
			continue
		}
		fresh, err := Compile(db, nodes[i])
		if err != nil {
			nodes[i].Pending = true
			nodes[i].LastError = "compile_failed"
			continue
		}
		fingerprint, err := snapshotFingerprint(fresh)
		if err != nil {
			return nil, err
		}
		nodes[i].Pending = nodes[i].LastSeen == 0 || nodes[i].DesiredRevision == 0 || nodes[i].DesiredRevision != nodes[i].AppliedRevision || fingerprint != nodes[i].DesiredFingerprint
		if nodes[i].TemplateTargetVersion > 0 && (nodes[i].TemplateTargetVersion != nodes[i].MaterializedTemplateVersion || nodes[i].TemplateTargetVersion != nodes[i].AppliedTemplateVersion) {
			nodes[i].Pending = true
		}
	}
	if err := db.Order("id").Find(&templates).Error; err != nil {
		return nil, err
	}
	if err := db.Order("node_id,id").Find(&inbounds).Error; err != nil {
		return nil, err
	}
	for i := range inbounds {
		if inbounds[i].ConnectionKey != "" {
			redactInbound(&inbounds[i])
		}
	}
	installations := []model.NodeInstallation{}
	jobs := []model.NodeJob{}
	if err := db.Find(&installations).Error; err != nil {
		return nil, err
	}
	for index := range installations {
		installations[index].HostFingerprint = sshFingerprint(installations[index].HostKey)
	}
	if err := db.Order("created desc, id desc").Limit(30).Find(&jobs).Error; err != nil {
		return nil, err
	}
	accessStates := []model.NodeAccessState{}
	if err := db.Find(&accessStates).Error; err != nil {
		return nil, err
	}
	tlsProfiles, err := listTLSProfiles(db)
	if err != nil {
		return nil, err
	}
	return map[string]interface{}{"nodeaccess": accessStates, "nodes": nodes, "nodetemplates": templates, "nodeinbounds": inbounds, "installations": installations, "nodejobs": jobs, "tls_profiles": tlsProfiles}, nil
}

func validPublishedAddress(value string) bool {
	if value == "" || strings.TrimSpace(value) != value {
		return false
	}
	if net.ParseIP(value) != nil {
		return true
	}
	if strings.HasPrefix(value, "[") && strings.HasSuffix(value, "]") {
		return net.ParseIP(value[1:len(value)-1]) != nil
	}
	if strings.ContainsAny(value, ": /?#@\\\t\r\n[]") {
		return false
	}
	ascii, err := idna.Lookup.ToASCII(strings.TrimSuffix(value, "."))
	if err != nil || len(ascii) > 253 {
		return false
	}
	for _, label := range strings.Split(ascii, ".") {
		if !validNodeDNSLabel(label) {
			return false
		}
	}
	return true
}

func validateManagementControl(node model.Node) error {
	u, err := url.Parse(node.Address)
	if err != nil || u.Scheme != "https" || u.Hostname() == "" || u.User != nil || u.RawQuery != "" || u.Fragment != "" {
		return errors.New("node management address must be an HTTPS origin without credentials/query")
	}
	if _, err := tls.X509KeyPair([]byte(node.ControlCertificate), []byte(node.ControlKey)); err != nil {
		return errors.New("invalid node controller certificate/key")
	}
	pool := x509.NewCertPool()
	if !pool.AppendCertsFromPEM([]byte(node.ControlCA)) {
		return errors.New("invalid node server CA")
	}
	return nil
}

func Save(object, action string, data json.RawMessage, actor string) error {
	if !IsObject(object) {
		return errors.New("unknown Nodes object")
	}
	return database.GetDB().Transaction(func(tx *gorm.DB) error {
		if action == "setoverrides" || action == "restoreversion" {
			return saveLiveAction(tx, object, action, data, actor)
		}
		id, deleteRequest, err := parseSaveIdentity(object, action, data)
		if err != nil {
			return err
		}
		switch object {
		case "nodetemplates":
			id, err = saveTemplateObject(tx, action, data, id)
		case "nodes":
			id, err = saveServerObject(tx, action, data, id, deleteRequest)
		}
		if err != nil {
			return err
		}
		return finishNodeSave(tx, object, action, id, deleteRequest, actor)
	})
}

func validNodeDNSLabel(label string) bool {
	if len(label) == 0 || len(label) > 63 || label[0] == '-' || label[len(label)-1] == '-' {
		return false
	}
	for _, c := range label {
		if !((c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || (c >= '0' && c <= '9') || c == '-') {
			return false
		}
	}
	return true
}
