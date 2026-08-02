package service

import (
	"crypto/rand"
	"encoding/hex"
	"time"
)

type RedeemCode struct {
	ID        int64
	Code      string
	Type      string
	Value     float64
	PaidValue float64
	Status    string
	UsedBy    *int64
	UsedAt    *time.Time
	Notes     string
	CreatedAt time.Time
	UpdatedAt time.Time

	GroupID      *int64
	GroupIDs     []int64
	ValidityDays int

	BatchID          *int64
	Purpose          string
	SalesStatus      string
	SoldAt           *time.Time
	SoldToNote       string
	ExternalOrderNo  string
	ExternalOrderURL string
	InternalNotes    string

	User  *User
	Group *Group
	Batch *RedeemCodeBatch
}

func (r *RedeemCode) IsUsed() bool {
	return r.Status == StatusUsed
}

func (r *RedeemCode) CanUse() bool {
	return r.Status == StatusUnused
}

func GenerateRedeemCode() (string, error) {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}

type RedeemCodeBatch struct {
	ID           int64
	Name         string
	Purpose      string
	FaceValue    float64
	Currency     string
	SalesChannel string
	ExternalURL  string
	Notes        string
	CreatedBy    *int64
	CreatedAt    time.Time
	UpdatedAt    time.Time
}
