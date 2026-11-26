package nodes

import (
	"encoding/json"
	"errors"
	"strconv"

	"gorm.io/gorm"
)

type listenerOptions struct {
	Port      json.RawMessage   `json:"listen_port"`
	Transport listenerTransport `json:"transport"`
	Network   string            `json:"network"`
}

type listenerSources struct {
	port                                                       json.RawMessage
	source, transportSource, typeSource                        ValueSource
	sourced, transportSourced, typeSourced, clearTransportType bool
}

func listenerEntrySources(entry SpecEntry, options listenerOptions) listenerSources {
	value := listenerSources{port: options.Port}
	value.source, value.sourced = entry.Sources[listenPortPath]
	value.transportSource, value.transportSourced = entry.Sources[transportPath]
	value.typeSource, value.typeSourced = entry.Sources[transportTypePath]
	return value
}

func (s *listenerSources) applyOverrides(options *listenerOptions, overrides []SpecOverride) error {
	for _, override := range overrides {
		switch override.Path {
		case "/network":
			options.Network = ""
			if override.Operation == "set" && (override.Source.Kind != "literal" || json.Unmarshal(override.Source.Value, &options.Network) != nil) {
				return errors.New("invalid node network")
			}
		case listenPortPath:
			s.source, s.sourced = override.Source, override.Operation == "set"
			s.port = nil
		case transportPath:
			s.transportSource, s.transportSourced = override.Source, override.Operation == "set"
			s.typeSourced = false
			options.Transport = listenerTransport{}
		case transportTypePath:
			s.typeSource, s.typeSourced = override.Source, override.Operation == "set"
			s.clearTransportType = !s.typeSourced
		}
	}
	return nil
}

func (s listenerSources) applyTransport(options *listenerOptions) error {
	if s.transportSourced && (s.transportSource.Kind != "literal" || json.Unmarshal(s.transportSource.Value, &options.Transport) != nil) {
		return errors.New("invalid_listener_transport_source")
	}
	if s.clearTransportType {
		options.Transport.Type = ""
	}
	if s.typeSourced && (s.typeSource.Kind != "literal" || json.Unmarshal(s.typeSource.Value, &options.Transport.Type) != nil) {
		return errors.New("invalid_listener_transport_source")
	}
	return nil
}

func (s listenerSources) resolvePort(tx *gorm.DB, nodeID uint, key string) (int, error) {
	port := s.port
	if s.sourced {
		switch {
		case s.source.Kind == "literal":
			port = s.source.Value
		case s.source.Kind == "node_generated" && s.source.Purpose == "listen_port":
			var err error
			port, err = loadGeneratedValue(tx, nodeID, key, "listen_port")
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return 0, nil
			}
			if err != nil {
				return 0, err
			}
		default:
			return 0, errors.New("invalid_listener_port_source")
		}
	}
	var value int
	if json.Unmarshal(port, &value) != nil || value < 1 || value > 65535 {
		return 0, errors.New("node_listener_port_required")
	}
	return value, nil
}

type listenerReservations struct {
	tx       *gorm.DB
	nodeID   uint
	host     string
	occupied map[listenerSocket]bool
	reserved map[string][]provisionListener
}

func (p listenerReservations) reserve(request listenerRequest) error {
	sockets := expandSocket(request.listenerSocket)
	for _, socket := range sockets {
		if p.occupied[socket] {
			return errListenerOccupied
		}
	}
	write := func(db *gorm.DB) error {
		for _, socket := range sockets {
			if err := reserveListenerPort(db, p.nodeID, p.host, request.Key, socket); err != nil {
				return err
			}
		}
		return nil
	}
	var err error
	if len(sockets) > 1 {
		err = p.tx.Transaction(write)
	} else {
		err = write(p.tx)
	}
	if err != nil {
		return err
	}
	for _, socket := range sockets {
		p.reserved[request.Key] = append(p.reserved[request.Key], provisionListener{request.Key, socket.Network, socket.Port})
	}
	return nil
}

func (p listenerReservations) reserveAutomatic(request listenerRequest) error {
	for _, port := range automaticListenerPorts {
		request.Port = port
		if err := p.reserve(request); err != nil {
			if errors.Is(err, errListenerReserved) || errors.Is(err, errListenerOccupied) {
				continue
			}
			return err
		}
		if err := storeGeneratedValue(p.tx, p.nodeID, request.Key, "listen_port", json.RawMessage(strconv.Itoa(port))); err != nil {
			return err
		}
		return nil
	}
	return errAutoPortsExhausted
}
