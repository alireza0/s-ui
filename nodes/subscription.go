package nodes

import (
	"maps"

	"github.com/alireza0/s-ui/database/model"
	"gorm.io/gorm"
)

type clientConnection struct {
	inbound *model.Inbound
	target  uint
}

// ClientInbounds supplies endpoints for link and Clash subscriptions.
func ClientInbounds(db *gorm.DB, client model.Client) ([]*model.Inbound, error) {
	connections, err := clientConnections(db, client)
	if err != nil {
		return nil, err
	}
	inbounds := make([]*model.Inbound, 0, len(connections))
	for _, connection := range connections {
		if connection.inbound.Type != "shadowtls" {
			inbounds = append(inbounds, connection.inbound)
		}
	}
	return inbounds, nil
}

type nativeTarget struct {
	id       uint
	outbound map[string]any
}

// NativeSubscription keeps transport references separate from the local inbound model.
type NativeSubscription struct {
	Inbounds   []*model.Inbound
	remote     map[*model.Inbound]uint
	targets    []nativeTarget
	transports map[uint][]map[string]any
	tags       map[string]bool
}

func NewNativeSubscription(db *gorm.DB, client model.Client) (*NativeSubscription, error) {
	connections, err := clientConnections(db, client)
	if err != nil {
		return nil, err
	}
	s := &NativeSubscription{
		Inbounds:   make([]*model.Inbound, 0, len(connections)),
		remote:     make(map[*model.Inbound]uint, len(connections)),
		transports: make(map[uint][]map[string]any),
	}
	for _, connection := range connections {
		s.Inbounds = append(s.Inbounds, connection.inbound)
		s.remote[connection.inbound] = connection.target
	}
	return s, nil
}

// Capture records rendered endpoints and keeps ShadowTLS out of the selector.
func (s *NativeSubscription) Capture(inbound *model.Inbound, outbound map[string]any, tags map[string]bool) bool {
	target, remote := s.remote[inbound]
	if !remote {
		return true
	}
	s.tags = tags
	switch inbound.Type {
	case "shadowtls":
		if target != 0 {
			s.transports[target] = append(s.transports[target], outbound)
		}
		return false
	case "vless":
		s.targets = append(s.targets, nativeTarget{inbound.Id, outbound})
	}
	return true
}

// AddTransports publishes each ShadowTLS layer once, with a selectable VLESS
// endpoint for every supported address pair.
func (s *NativeSubscription) AddTransports(outbounds *[]map[string]any, tags *[]string, uniqueTag func(string, map[string]bool) string) {
	added := map[string]bool{}
	for _, target := range s.targets {
		targetTag, _ := target.outbound["tag"].(string)
		for _, layer := range s.transports[target.id] {
			layerTag, _ := layer["tag"].(string)
			if !added[layerTag] {
				*outbounds = append(*outbounds, layer)
				added[layerTag] = true
			}
			tag := uniqueTag(targetTag+" · "+layerTag, s.tags)
			s.tags[tag] = true
			chained := maps.Clone(target.outbound)
			chained["detour"], chained["tag"] = layerTag, tag
			*outbounds = append(*outbounds, chained)
			*tags = append(*tags, tag)
		}
	}
}
