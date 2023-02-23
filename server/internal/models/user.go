package model

import "encoding/json"

type UserDataDto struct {
	UserID        string       `json:"userId"`
	Email         string       `json:"email"`
	CalendarToken string       `json:"calendarToken"`
	Settings      UserSettings `json:"settings"`
	AllowNotify   string       `json:"allowNotify"`
}

type UpsertUser struct {
	UserID        string       `json:"userId"`
	Email         string       `json:"email"`
	CalendarToken string       `json:"calendarToken"`
	Setting       UserSettings `json:"settings"`
	AllowNotify   string       `json:"allowNotify"` // Y or N
	EncryptSecret string       `json:"encryptSecret"`
}

type UpdateUser struct {
	Email         string       `json:"email"`
	CalendarToken string       `json:"calendarToken"`
	Setting       UserSettings `json:"settings"`
	AllowNotify   string       `json:"allowNotify"` // Y or N
}

type UserSettings struct {
	TimeNotiBeforeEvent int `json:"timeNotiBeforeEvent"`
}

type ListUsersOption struct {
	Page          int    `json:"page"`
	Limit         int    `json:"limit"`
	SortBy        string `json:"sortBy"`
	SortDirection string `json:"sortDirection"`
	AllowNotify   string `json:"allowNotify"`
}

type ListUsersResult struct {
	Users      []UserDataDto `json:"users"`
	TotalRows  int64         `json:"total_rows"`
	TotalPages int           `json:"total_pages"`
}

func (u UserSettings) String() (string, error) {
	bytes, err := json.Marshal(u)
	if err != nil {
		return "", err
	}
	return string(bytes), nil
}
