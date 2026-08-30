package model

import (
	"encoding/json"
)

type Endpoint struct {
	Id   uint   `json:"id" form:"id" gorm:"primaryKey;autoIncrement"`
	Type string `json:"type" form:"type"`
	Tag  string `json:"tag" form:"tag" gorm:"unique"`

	// Foreign key to tls table. Only the endpoint types that terminate or
	// present TLS themselves use it; see projectEndpointTLS.
	TlsId uint `json:"tls_id" form:"tls_id"`
	Tls   *Tls `json:"tls" form:"tls" gorm:"foreignKey:TlsId;references:Id"`

	Options json.RawMessage `json:"-" form:"-"`
	Ext     json.RawMessage `json:"ext" form:"ext"`
}

func (o *Endpoint) UnmarshalJSON(data []byte) error {
	var err error
	var raw map[string]interface{}
	if err = json.Unmarshal(data, &raw); err != nil {
		return err
	}

	// Extract fixed fields and store the rest in Options
	if val, exists := raw["id"].(float64); exists {
		o.Id = uint(val)
	}
	delete(raw, "id")
	o.Type, _ = raw["type"].(string)
	delete(raw, "type")
	o.Tag = raw["tag"].(string)
	delete(raw, "tag")
	if val, exists := raw["tls_id"].(float64); exists {
		o.TlsId = uint(val)
	}
	delete(raw, "tls_id")
	delete(raw, "tls")
	o.Ext, _ = json.MarshalIndent(raw["ext"], "", "  ")
	delete(raw, "ext")

	// Remaining fields
	o.Options, err = json.MarshalIndent(raw, "", "  ")
	return err
}

// MarshalJSON customizes marshalling
func (o Endpoint) MarshalJSON() ([]byte, error) {
	// Combine fixed fields and dynamic fields into one map
	combined := make(map[string]interface{})
	switch o.Type {
	case "warp":
		combined["type"] = "wireguard"
	default:
		combined["type"] = o.Type
	}
	combined["tag"] = o.Tag

	if o.Options != nil {
		var restFields map[string]json.RawMessage
		if err := json.Unmarshal(o.Options, &restFields); err != nil {
			return nil, err
		}

		for k, v := range restFields {
			combined[k] = v
		}
	}

	// A referenced TLS config is the explicit choice, so it replaces whatever
	// TLS fields the endpoint carries inline rather than losing to them.
	if o.Tls != nil {
		projected, err := projectEndpointTLS(o.Type, o.Tls)
		if err != nil {
			return nil, err
		}
		if projected != nil {
			combined["tls"] = projected
		} else {
			delete(combined, "tls")
		}
	}

	return json.Marshal(combined)
}
