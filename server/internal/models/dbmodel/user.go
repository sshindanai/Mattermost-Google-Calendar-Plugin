package dbmodel

import (
	"time"
)

type Users struct {
	ID            string    `json:"id" gorm:"primaryKey;type:VARCHAR(255);not null;uniqueIndex"`
	Email         string    `json:"email" gorm:"type:VARCHAR(127);index"`
	CalendarToken string    `json:"calendar_token" gorm:"type:TEXT"`
	Settings      string    `json:"settings" gorm:"type:TEXT"`
	AllowNotify   string    `json:"allow_notify" gorm:"type:VARCHAR(1);default:'Y';index"`
	CreatedAt     time.Time `json:"created_at"`
	UpdatedAt     time.Time `json:"updated_at"`
	TccState      int       `json:"tcc_state" gorm:"type:INT;default:0"`
}

func (*Users) TableName() string {
	return "users"
}
