package models

import (
	"time"

	"github.com/google/uuid"
)

type H402PendingRequests struct {
	ID                uuid.UUID  `gorm:"type:char(36);primaryKey" json:"id"`
	AgentID           uuid.UUID  `gorm:"type:char(36);index;not null;constraint:OnDelete:CASCADE" json:"agentId"`
	UserID            uuid.UUID  `gorm:"type:char(36);index;not null;constraint:OnDelete:CASCADE" json:"userId"`
	Status            string     `gorm:"type:varchar(20);not null;default:'Pending';index" json:"status"` // "Pending", "Approved", or "Cancelled"
	SelectedRequestID *uuid.UUID `gorm:"type:char(36);constraint:OnDelete:CASCADE" json:"selectedPaymentId,omitempty"`
	PaymentHeader     *string    `gorm:"type:text" json:"paymentHeader,omitempty"`
	CreatedAt         time.Time  `json:"createdAt"`
	UpdatedAt         time.Time  `json:"updatedAt"`

	Agent Agent `gorm:"foreignKey:AgentID;references:ID;constraint:OnDelete:CASCADE" json:"-"`
	User  User  `gorm:"foreignKey:UserID;references:ID;constraint:OnDelete:CASCADE" json:"-"`
}
