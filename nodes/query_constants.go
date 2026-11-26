package nodes

const (
	whereNodeID          = "node_id = ?"
	whereNodeVersion     = "node_id = ? AND version = ?"
	whereTemplateID      = "template_id = ?"
	whereTemplateVersion = "template_id = ? AND version = ?"
	whereRequestKey      = "request_key = ?"
	listenPortPath       = "/listen_port"
	transportPath        = "/transport"
	transportTypePath    = "/transport/type"
	cloudflareZonesPath  = "/zones/"
)
