package model

import "encoding/json"

// Link represents a subscription link
type Link struct {
	Type   string `json:"type"`
	Remark string `json:"remark"`
	Uri    string `json:"uri"`
}

// MarshalJSON marshals Link to JSON
func (l *Link) MarshalJSON() ([]byte, error) {
	type Alias Link
	return json.Marshal(&struct{ *Alias }{
		Alias: (*Alias)(l),
	})
}

// UnmarshalJSON unmarshals JSON to Link
func (l *Link) UnmarshalJSON(data []byte) error {
	type Alias Link
	aux := &struct{ *Alias }{
		Alias: (*Alias)(l),
	}
	return json.Unmarshal(data, aux)
}
