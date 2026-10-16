package nodes

import (
	"gowave/internal/database"
	"gowave/internal/notify"

	"gorm.io/gorm"
)

// emitNode enqueues a node.* notification with the API node payload.
func (s *Service) emitNode(tx *gorm.DB, node *database.Node, name string) error {
	return notify.Emit(tx, notify.Event{Name: name, Data: formatNodeWith(tx, s, node)})
}

// emitNodeByUUID reloads the node through tx before notifying.
func (s *Service) emitNodeByUUID(tx *gorm.DB, uuid, name string) error {
	var node database.Node
	if err := tx.Where("uuid = ?", uuid).First(&node).Error; err != nil {
		return err
	}
	return s.emitNode(tx, &node, name)
}

// EventData formats a node like the API for notifications sent elsewhere.
func (s *Service) EventData(node *database.Node) interface{} {
	return formatNodeWith(s.db, s, node)
}
