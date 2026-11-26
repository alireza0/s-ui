package nodes

import (
	"github.com/alireza0/s-ui/database/model"
	"gorm.io/gorm"
)

func accountingClients(tx *gorm.DB, node model.Node, includeRevoked bool) ([]model.Client, error) {
	assignments := tx.Model(&model.ClientNode{}).Select("client_id").Where(whereNodeID, node.Id)
	if !includeRevoked {
		assignments = assignments.Where("active = ?", true)
	}
	var clients []model.Client
	err := tx.Where("clients.id IN (?)", assignments).Find(&clients).Error
	return clients, err
}
