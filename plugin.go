package main

import (
	_ "embed"
	"encoding/json"

	"github.com/mattermost/mattermost-server/v5/model"
)

//go:embed plugin.json
var pluginData []byte

var Manifest *model.Manifest

func init() {
	manifest := &model.Manifest{}
	if err := json.Unmarshal(pluginData, manifest); err != nil {
		panic(err)
	}
	Manifest = manifest
}
