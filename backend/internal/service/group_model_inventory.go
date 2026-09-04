package service

import "context"

// AccountModelInventory contains only model declarations, never authentication
// secrets or transient health. A failed account still declares its models;
// account selection handles health after the billing group has been chosen.
type AccountModelInventory struct {
	Platform     string
	Type         string
	ModelMapping map[string]any
}

type GroupModelInventoryReader interface {
	ListGroupModelInventory(context.Context, int64) ([]AccountModelInventory, error)
}
