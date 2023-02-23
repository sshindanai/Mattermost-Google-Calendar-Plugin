package model

import (
	dbmodel "github.com/sshindanai/Mattermost-Google-Calendar-Plugin/server/internal/models/dbmodel"
)

type Lookups dbmodel.Lookups

type LookupsResponse struct {
	Key   string `json:"key"`
	Value string `json:"value"`
}

type LookupsRequest struct {
	UserID string `json:"userId"`
	Key    string `json:"key"`
}

func (l LookupsRequest) IsValid() bool {
	return l.UserID != "" && l.Key != ""
}
