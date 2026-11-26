package database

import (
	"github.com/alireza0/s-ui/database/model"

	"gorm.io/gorm"
)

// table ties a model to the code that copies its rows. The list used to be
// written out twice, in InitDB and in GetDb, and drifted: Service and Tokens
// were migrated but never backed up.
type table struct {
	// name is what the backup endpoint's exclude parameter matches on.
	name  string
	model any
	// copyRows moves every row of this table from src to dst.
	copyRows func(src, dst *gorm.DB) error
}

// schema is the whole database: adding a model here migrates it and backs it up.
func schema() []table {
	return append([]table{
		{"settings", &model.Setting{}, copyRows[model.Setting]},
		{"tls", &model.Tls{}, copyRows[model.Tls]},
		{"inbounds", &model.Inbound{}, copyRows[model.Inbound]},
		{"outbounds", &model.Outbound{}, copyRows[model.Outbound]},
		{"services", &model.Service{}, copyRows[model.Service]},
		{"endpoints", &model.Endpoint{}, copyRows[model.Endpoint]},
		{"users", &model.User{}, copyRows[model.User]},
		{"tokens", &model.Tokens{}, copyRows[model.Tokens]},
		{"stats", &model.Stats{}, copyRows[model.Stats]},
		{"clients", &model.Client{}, copyRows[model.Client]},
		{"changes", &model.Changes{}, copyRows[model.Changes]},
	}, nodeSchema()...)
}

// schemaModels returns the models in schema order, for AutoMigrate.
func schemaModels() []any {
	tables := schema()
	models := make([]any, 0, len(tables))
	for _, t := range tables {
		models = append(models, t.model)
	}
	return models
}

func copyRows[T any](src, dst *gorm.DB) error {
	var rows []T
	if err := src.Model(new(T)).Scan(&rows).Error; err != nil {
		return err
	}
	if len(rows) == 0 {
		return nil
	}
	return dst.Save(rows).Error
}
