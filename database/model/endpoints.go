package model

import (
	"encoding/json"
)

// Endpoint has no tls_id, unlike Inbound and Service. OpenConnect and OpenVPN
// each define their own TLS options with their own field names, their own
// vocabulary (an OpenVPN server names the client CA `client_certificate`) and
// settings the panel's TLS config cannot express at all, control_wrap among
// them. A shared template projected onto them was misleading at best and, for
// OpenVPN, never produced a config sing-box would accept. Those endpoints carry
// their own `tls` object in Options instead, written in sing-box's own names.
type Endpoint struct {
	Id   uint   `json:"id" form:"id" gorm:"primaryKey;autoIncrement"`
	Type string `json:"type" form:"type"`
	Tag  string `json:"tag" form:"tag" gorm:"unique"`

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
	o.Tag, _ = raw["tag"].(string)
	delete(raw, "tag")
	// Dropped rather than stored: an older panel sent tls_id alongside the
	// endpoint, and sing-box rejects the unknown key. `tls` is kept, since it
	// is now the endpoint's own.
	delete(raw, "tls_id")
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

	return json.Marshal(combined)
}
