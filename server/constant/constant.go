package constant

import (
	model "github.com/sshindanai/Mattermost-Google-Calendar-Plugin/server/internal/models"
)

const (
	// Metadata
	GOOGLE_CALENDAR_API_URL = "https://www.googleapis.com/auth/calendar"
	MATTERMOST_USER_KEY     = "Mattermost-User-ID"

	// Date format
	DATE_FORMAT           = "Monday, January 2, 2006"
	TIME_FORMAT           = "3:04 PM MST"
	CUSTOM_FORMAT         = "2006-01-02@15:04"
	CUSTOM_FORMAT_NO_TIME = "2006-01-02"

	// Command
	MAIN_CMD       = "/calendar"
	CONNECT_CMD    = "connect"
	CREATE_CMD     = "create"
	SUMMARY_CMD    = "summary"
	SETTINGS_CMD   = "settings"
	NEXT_CMD       = "next"
	DISCONNECT_CMD = "disconnect"
	HELP_CMD       = "help"

	// config
	ALLOW_NOTIFY     = "Y"
	NOT_ALLOW_NOTIFY = "N"

	// Calendar  ID
	PRIMARY_CALENDAR_ID = "primary"

	// Error message
	INTERNAL_ERR_USER_NOT_FOUND = "user not found"
	ERR_CONNECT_FIRST           = "Please connect your google calendar with command => `/calendar connect`"

	// Event response statutus
	EV_STATUS_NEED_ACTION = "needsAction"
	EV_STATUS_ACCEPTED    = "accepted"
	EV_STATUS_DECLINED    = "declined"
	EV_STATUS_TENTATIVE   = "tentative"
	EV_STATUS_CANCELLED   = "cancelled"

	// Key
	EVENTS_KEY        = "events"
	WATCH_TOKEN_KEY   = "watch_token"
	WATCH_CHANNEL_KEY = "watch_channel"
	SYNC_TOKEN_KEY    = "sync_token"

	// SITE_URL = "https://51c9-180-180-58-99.ap.ngrok.io"
	EMAIL_REGEX = `^[a-z0-9._%+\-]+@[a-z0-9.\-]+\.[a-z]{2,4}$`
	// EMAIL_REGEX  = `([!#-'*+/-9=?A-Z^-~-]+(\.[!#-'*+/-9=?A-Z^-~-]+)*|\"\(\[\]!#-[^-~ \t]|(\\[\t -~]))+\")@([!#-'*+/-9=?A-Z^-~-]+(\.[!#-'*+/-9=?A-Z^-~-]+)*|\[[\t -Z^-~]*])`
)

var (
	DefaultUserSettings = model.UserSettings{
		TimeNotiBeforeEvent: 10,
	}
)
