package models

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// OAuth platforms
const (
	PlatformGmail          = "gmail"
	PlatformGithub         = "github"
	PlatformGoogleCalendar = "google-calendar"
	// Add more platforms as needed
)

type OAuth struct {
	ID           uuid.UUID `gorm:"type:char(36);primaryKey" json:"id"`
	UserID       uuid.UUID `gorm:"type:char(36);index;not null;constraint:OnDelete:CASCADE" json:"userId"`
	Platform     string    `gorm:"type:varchar(50);not null;index" json:"platform"` // gmail, github, etc
	AccessToken  string    `gorm:"type:text;not null" json:"-"`                     // Don't expose in JSON
	RefreshToken string    `gorm:"type:text" json:"-"`                              // Don't expose in JSON (optional for some platforms)
	TokenExpiry  time.Time `gorm:"not null" json:"tokenExpiry"`
	Email        string    `gorm:"type:varchar(255)" json:"email"` // Email/username for the platform
	IsActive     bool      `gorm:"type:boolean;default:true;not null" json:"isActive"`
	Scopes       string    `gorm:"type:text" json:"scopes"`              // Comma-separated scopes
	ExtraData    string    `gorm:"type:json" json:"extraData,omitempty"` // Platform-specific data as JSON
	CreatedAt    time.Time `json:"createdAt"`
	UpdatedAt    time.Time `json:"updatedAt"`

	// Foreign key relationships
	User User `gorm:"foreignKey:UserID;references:ID;constraint:OnDelete:CASCADE" json:"-"`
}

func (o *OAuth) BeforeCreate(tx *gorm.DB) (err error) {
	o.ID = uuid.New()
	return
}

// IsTokenExpired checks if the access token has expired
func (o *OAuth) IsTokenExpired() bool {
	return time.Now().After(o.TokenExpiry)
}

// NeedsRefresh checks if the token needs to be refreshed (expires within 5 minutes)
func (o *OAuth) NeedsRefresh() bool {
	return time.Now().Add(5 * time.Minute).After(o.TokenExpiry)
}

// IsGmail checks if this OAuth is for Gmail platform
func (o *OAuth) IsGmail() bool {
	return o.Platform == PlatformGmail
}

// IsGithub checks if this OAuth is for GitHub platform
func (o *OAuth) IsGithub() bool {
	return o.Platform == PlatformGithub
}

// IsGoogleCalendar checks if this OAuth is for Google Calendar platform
func (o *OAuth) IsGoogleCalendar() bool {
	return o.Platform == PlatformGoogleCalendar
}
