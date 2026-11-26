package nodes

import (
	"encoding/json"

	"github.com/alireza0/s-ui/database"
	"github.com/alireza0/s-ui/database/model"
)

// ReadWorkspace is a read-only editor projection. Generated material and delivery
// payloads never enter it; looking at a node cannot materialize or deploy it.
func ReadWorkspace(id uint) (map[string]interface{}, error) {
	db := database.GetDB()
	var node model.Node
	if err := db.First(&node, id).Error; err != nil {
		return nil, err
	}
	var spec *TemplateSpec
	if node.TemplateTargetVersion > 0 {
		s, err := liveVersion(db, node)
		if err != nil {
			return nil, err
		}
		spec = &s
	}
	overrides := []model.NodeOverride{}
	connections := []model.NodeInbound{}
	tasks := []model.NodeTask{}
	if err := db.Where("node_id = ? AND archived = false", id).Order("connection_key,path").Find(&overrides).Error; err != nil {
		return nil, err
	}
	if err := db.Where(whereNodeID, id).Order("id").Find(&connections).Error; err != nil {
		return nil, err
	}
	for i := range connections {
		redactInbound(&connections[i])
	}
	if err := db.Where(whereNodeID, id).Find(&tasks).Error; err != nil {
		return nil, err
	}
	return map[string]interface{}{"spec": spec, "overrides": overrides, "connections": connections, "tasks": tasks}, nil
}

func redactInbound(inbound *model.NodeInbound) {
	for _, raw := range []*json.RawMessage{&inbound.Options, &inbound.ClientOptions} {
		var value interface{}
		if len(*raw) != 0 && json.Unmarshal(*raw, &value) == nil {
			redactWorkspace(value)
			*raw, _ = json.Marshal(value)
		}
	}
}

func redactWorkspace(value interface{}) {
	switch v := value.(type) {
	case map[string]interface{}:
		for key, item := range v {
			switch key {
			case "private_key", "key", "certificate", "api_key", "api_token", "password", "uuid", "users", "token":
				v[key] = "[managed]"
			default:
				redactWorkspace(item)
			}
		}
	case []interface{}:
		for _, item := range v {
			redactWorkspace(item)
		}
	}
}
