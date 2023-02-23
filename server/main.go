package main

import (
	"github.com/sshindanai/Mattermost-Google-Calendar-Plugin/server/plugin"

	pluginapi "github.com/mattermost/mattermost-server/v5/plugin"
)

func main() {
	pluginapi.ClientMain(&plugin.Plugin{})
}
