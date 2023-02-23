package dbmodel

import "time"

type Lookups struct {
	ID        int       `json:"id"`
	UserID    string    `json:"user_id"`
	Key       string    `json:"key"`
	Value     string    `json:"value"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
	TccState  int       `json:"tcc_state"`
}

func (*Lookups) TableName() string {
	return "lookups"
}
