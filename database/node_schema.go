package database

import "github.com/alireza0/s-ui/database/model"

func nodeSchema() []table {
	return []table{
		{"node_templates", &model.NodeTemplate{}, copyRows[model.NodeTemplate]},
		{"node_template_members", &model.NodeTemplateMember{}, copyRows[model.NodeTemplateMember]},
		{"nodes", &model.Node{}, copyRows[model.Node]},
		{"node_inbounds", &model.NodeInbound{}, copyRows[model.NodeInbound]},
		{"node_installations", &model.NodeInstallation{}, copyRows[model.NodeInstallation]},
		{"node_jobs", &model.NodeJob{}, copyRows[model.NodeJob]},
		{"client_nodes", &model.ClientNode{}, copyRows[model.ClientNode]},
		{"node_template_versions", &model.NodeTemplateVersion{}, copyRows[model.NodeTemplateVersion]},
		{"node_overrides", &model.NodeOverride{}, copyRows[model.NodeOverride]},
		{"node_generated_materials", &model.NodeGeneratedMaterial{}, copyRows[model.NodeGeneratedMaterial]},
		{"node_tasks", &model.NodeTask{}, copyRows[model.NodeTask]},
		{"node_access_states", &model.NodeAccessState{}, copyRows[model.NodeAccessState]},
		{"node_client_traffics", &model.NodeClientTraffic{}, copyRows[model.NodeClientTraffic]},
		{"node_deployments", &model.NodeDeployment{}, copyRows[model.NodeDeployment]},
		{"node_port_reservations", &model.NodePortReservation{}, copyRows[model.NodePortReservation]},
		{"node_dns_profiles", &model.NodeDNSProfile{}, copyRows[model.NodeDNSProfile]},
		{"node_domain_bindings", &model.NodeDomainBinding{}, copyRows[model.NodeDomainBinding]},
		{"node_dns_records", &model.NodeDNSRecord{}, copyRows[model.NodeDNSRecord]},
		{"node_certificate_bindings", &model.NodeCertificateBinding{}, copyRows[model.NodeCertificateBinding]},
		{"node_certificate_versions", &model.NodeCertificateVersion{}, copyRows[model.NodeCertificateVersion]},
	}
}
