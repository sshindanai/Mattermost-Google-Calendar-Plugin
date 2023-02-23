package plugin

import (
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"path/filepath"
	"sync"

	"github.com/gorilla/mux"
	"github.com/mattermost/mattermost-server/v5/model"
	"github.com/mattermost/mattermost-server/v5/plugin"
	"github.com/pkg/errors"
	models "github.com/sshindanai/Mattermost-Google-Calendar-Plugin/server/internal/models"
)

// Plugin implements the interface expected by the Mattermost server to communicate
// between the server and plugin processes.
//
// Lifecycle:
//          Enable plugin: OnConfigurationChange --> OnActivate --> OnConfigurationChange --> ServeHTTP --> Started
//          Incoming command --> ExecuteCommand --> Response

type Plugin struct {
	configuration *configuration
	plugin.MattermostPlugin
	botID             string
	configurationLock sync.RWMutex
	services          *InternalService
	router            *mux.Router
}

// ServeHTTP allows the plugin to implement the http.Handler interface. Requests destined for the
// /plugins/{id} path will be routed to the plugin.
//
// The Mattermost-User-Id header will be present if (and only if) the request is by an
// authenticated user.
func (p *Plugin) ServeHTTP(c *plugin.Context, w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	defer func() {
		if err := recover(); err != nil {
			p.API.LogInfo("recovered server", "error", err)
			w.WriteHeader(http.StatusInternalServerError)
			fmt.Fprintf(w, "error: %s", err)
		}
	}()
	p.router.ServeHTTP(w, r)
}

//OnActivate function ensures what bot does when become actived
func (p *Plugin) OnActivate() error {
	p.API.LogInfo("Google Calendar Plugin activating...")

	// get command
	command, err := p.getCommand()
	if err != nil {
		return errors.Wrap(err, "failed to get command")
	}

	// register command
	if err = p.API.RegisterCommand(command); err != nil {
		return err
	}
	p.API.LogInfo("Google Calendar Plugin command was registered")

	botID, err := p.Helpers.EnsureBot(&model.Bot{
		Username:    "google.calendar",
		DisplayName: "Google Calendar",
		Description: "Created by the Google Calendar plugin.",
	})
	if err != nil {
		return errors.Wrap(err, "failed to ensure google calendar bot")
	}
	p.botID = botID
	p.API.LogInfo("Google Calendar Plugin bot was created")

	bundlePath, err := p.API.GetBundlePath()
	if err != nil {
		return errors.Wrap(err, "couldn't get bundle path")
	}

	profileImage, err := ioutil.ReadFile(filepath.Join(bundlePath, "assets", "profile.png"))
	if err != nil {
		return errors.Wrap(err, "couldn't read profile image")
	}

	appErr := p.API.SetProfileImage(botID, profileImage)
	if appErr != nil {
		return errors.Wrap(appErr, "couldn't set profile image")
	}
	p.API.LogInfo("Google Calendar Plugin profile image was set")

	// init internal service
	p.services = p.NewInternalService()
	p.API.LogInfo("Google Calendar Plugin internal service was created")

	if err := p.SyncUserData(); err != nil {
		return errors.Wrap(err, "failed to load user data")
	}
	p.registerRouter()
	p.notifyCronJob()
	p.API.LogInfo("Google Calendar Plugin activate successfully")
	return nil
}

func (p *Plugin) OnDeactivate() error {
	p.API.LogInfo("Google Calendar Plugin deactivating...")
	// close db
	if err := p.CloseDb(); err != nil {
		p.API.LogError("failed to close database", "err", err)
		return err
	}
	p.API.LogInfo("Google Calendar Plugin database was closed")
	p.API.LogInfo("Google Calendar Plugin deactivate successfully")
	return nil
}

func (p *Plugin) SyncUserData() error {
	p.API.LogInfo("Google Calendar Plugin loading user data...")
	page := 1
	limit := 100
	secret := p.getConfiguration().EncryptionSecret

	for {
		result, err := p.services.userService.List(secret, models.ListUsersOption{
			Page:  page,
			Limit: limit,
		})
		if err != nil {
			p.API.LogError("failed to get users", "err", err)
			return err
		}
		if len(result.Users) == 0 {
			break
		}
		var wg sync.WaitGroup
		wg.Add(len(result.Users))
		maxGoroutines := 20
		queues := make(chan struct{}, maxGoroutines)
		for _, user := range result.Users {
			queues <- struct{}{}
			go func(user models.UserDataDto) {
				defer func() {
					wg.Done()
					<-queues
				}()
				p.SetupUserSync(user)
			}(user)
		}
		wg.Wait()
		page++
	}

	return nil
}

func (p *Plugin) SetupUserSync(user models.UserDataDto) {
	if err := p.CalendarSyncV2(user); err != nil {
		p.API.LogError("failed to sync calendar", "err", err)
		log.Fatal(err)
	}
	p.API.LogInfo("Google Calendar Plugin user data was synced", "user_id", user.UserID)
}
