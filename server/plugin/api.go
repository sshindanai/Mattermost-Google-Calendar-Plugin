package plugin

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"github.com/gorilla/mux"
	"github.com/mattermost/mattermost-server/v5/model"
	"github.com/sshindanai/Mattermost-Google-Calendar-Plugin/server/constant"
	"github.com/sshindanai/Mattermost-Google-Calendar-Plugin/server/helper"
	models "github.com/sshindanai/Mattermost-Google-Calendar-Plugin/server/internal/models"
	"golang.org/x/oauth2"
	"google.golang.org/api/calendar/v3"
)

func (p *Plugin) registerRouter() {
	router := mux.NewRouter()

	// register
	router.HandleFunc("/oauth/connect", p.connectCalendar)
	router.HandleFunc("/oauth/complete", p.completeCalendar)
	router.HandleFunc("/delete", p.deleteEvent)
	router.HandleFunc("/handleresponse", p.handleEventResponse)
	router.HandleFunc("/watch", p.watchCalendar)
	router.HandleFunc("/settings", p.setSettings)
	router.HandleFunc("/disconnect", p.disconnectCalendar)
	p.router = router
}

func (p *Plugin) connectCalendar(w http.ResponseWriter, r *http.Request) {
	authUserID := r.Header.Get(constant.MATTERMOST_USER_KEY)
	if authUserID == "" {
		http.Error(w, "Not authorized", http.StatusUnauthorized)
		return
	}

	state := fmt.Sprintf("%v_%v", model.NewId()[10], authUserID)

	// make sure by delete state
	if err := p.services.stateService.Delete(authUserID); err != nil {
		http.Error(w, "Failed to save state", http.StatusBadRequest)
		return
	}

	if err := p.services.stateService.Create(authUserID, state); err != nil {
		http.Error(w, "Failed to save state", http.StatusBadRequest)
		return
	}

	calendarConfig := p.CalendarConfig()
	url := calendarConfig.AuthCodeURL(state, oauth2.AccessTypeOffline, oauth2.ApprovalForce)
	http.Redirect(w, r, url, http.StatusTemporaryRedirect)
}

func (p *Plugin) completeCalendar(w http.ResponseWriter, r *http.Request) {
	html := `
	<!DOCTYPE html>
	<html>
		<head>
			<script>
				window.close();
			</script>
		</head>
		<body>
			<p>Completed connecting to Google Calendar. Please close this window.</p>
		</body>
	</html>
	`
	authUserID := r.Header.Get(constant.MATTERMOST_USER_KEY)
	state := r.FormValue("state")
	code := r.FormValue("code")
	userID := strings.Split(state, "_")[1]
	calendarConfig := p.CalendarConfig()
	if authUserID == "" || userID != authUserID {
		if err := p.services.stateService.Delete(userID); err != nil {
			http.Error(w, "Failed to delete state", http.StatusBadRequest)
			return
		}
		http.Error(w, "Not authorized", http.StatusUnauthorized)
		return
	}
	storedState, err := p.services.stateService.Get(userID)
	if err != nil {
		if err := p.services.stateService.Delete(userID); err != nil {
			http.Error(w, "Failed to delete state", http.StatusBadRequest)
			return
		}
		http.Error(w, "Failed to get state", http.StatusBadRequest)
		return
	}

	if string(storedState) != state {
		if err := p.services.stateService.Delete(userID); err != nil {
			http.Error(w, "Failed to delete state", http.StatusBadRequest)
			return
		}
		http.Error(w, "Invalid state", http.StatusBadRequest)
		return
	}

	if err := p.services.stateService.Delete(userID); err != nil {
		http.Error(w, "Failed to delete state", http.StatusBadRequest)
		return
	}

	token, err := calendarConfig.Exchange(context.Background(), code)
	if err != nil {
		http.Error(w, "Error setting up Config Exchange", http.StatusBadRequest)
		return
	}

	tokenJSON, err := json.Marshal(token)
	if err != nil {
		http.Error(w, "Invalid token marshal in completeCalendar", http.StatusBadRequest)
		return
	}

	// get email
	mattermostUser, appErr := p.API.GetUser(authUserID)
	if appErr != nil {
		http.Error(w, "Invalid user id in completeCalendar", http.StatusBadRequest)
		return
	}
	secret := p.getConfiguration().EncryptionSecret
	encryptedToken, err := helper.Encrypt(string(tokenJSON), secret)
	if err != nil {
		http.Error(w, "Invalid token encrypt in completeCalendar", http.StatusBadRequest)
		return
	}

	// persist token to db
	user, isUpdate, err := p.services.userService.UpsertUserToken(models.UpsertUser{
		UserID:        userID,
		CalendarToken: encryptedToken,
		Email:         mattermostUser.Email,
		EncryptSecret: secret,
	})
	if err != nil {
		http.Error(w, "failed to set token", http.StatusInternalServerError)
		return
	}

	if err := p.CalendarSyncV2(*user); err != nil {
		http.Error(w, "failed sync fresh calender", http.StatusInternalServerError)
		return
	}
	if isUpdate {
		if err = p.stopWatch(user.UserID); err != nil {
			http.Error(w, "failed to stop watch", http.StatusInternalServerError)
			return
		}
	}

	if err = p.setupCalendarWatchV2(*user); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	// Post intro post
	message := "#### Welcome to the Mattermost Google Calendar Plugin!\n" +
		"You've successfully connected your Mattermost account to your Google Calendar.\n" +
		"Please type **/calendar help** to understand how to user this plugin. "

	if err := p.CreateBotDMPost(userID, message); err != nil {
		p.API.LogError("Failed to post intro message", "err", err.Error())
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	fmt.Fprint(w, html)
}

// func (p *Plugin) createEvent(w http.ResponseWriter, r *http.Request) {
// 	var req model.SubmitDialogRequest
// 	if err := Decode(r.Body, &req); err != nil {
// 		return
// 	}
// 	submissionBytes, err := json.Marshal(req.Submission)
// 	if err != nil {
// 		return
// 	}
// 	var eventReq CreateEventDialog
// 	if err := json.Unmarshal(submissionBytes, &eventReq); err != nil {
// 		return
// 	}
// 	userID := req.UserId
// 	// validate time format
// 	if _, err := time.Parse(constant.CUSTOM_FORMAT, eventReq.StartDateTime); err != nil {
// 		if appErr := p.CreateBotDMPost(userID, fmt.Sprintf("Start time is invalid format-[%v]", eventReq.StartDateTime)); appErr != nil {
// 			p.API.LogError("Error creating bot post", "apErr", appErr.Error())
// 			return
// 		}
// 		return
// 	}
// 	if _, err := time.Parse(constant.CUSTOM_FORMAT, eventReq.EndDateTime); err != nil {
// 		if appErr := p.CreateBotDMPost(userID, fmt.Sprintf("End time is invalid format-[%v]", eventReq.EndDateTime)); appErr != nil {
// 			p.API.LogError("Error creating bot post", "apErr", appErr.Error())
// 			return
// 		}
// 		return
// 	}

// 	cal, err := p.getCalendarService(userID)
// 	if err != nil {
// 		if err.Error() == constant.INTERNAL_ERR_USER_NOT_FOUND {
// 			// tell user to connect first
// 			if appErr := p.CreateBotDMPost(userID, constant.ERR_CONNECT_FIRST); appErr != nil {
// 				p.API.LogError("Error creating bot post", "apErr", appErr.Error())
// 				return
// 			}
// 		}

// 		p.API.LogError("Error getting calendar service", "err", err.Error())
// 		return
// 	}

// 	location, err := p.getPrimaryCalendarLocation(userID)
// 	if err != nil {
// 		p.API.LogError("Error getting primary calendar location", "err", err.Error())
// 		return
// 	}

// 	startTime, err := time.ParseInLocation(constant.CUSTOM_FORMAT, eventReq.StartDateTime, location)
// 	if err != nil {
// 		p.API.LogError("Error parsing start time", "err", err.Error())
// 		return
// 	}
// 	endTime, err := time.ParseInLocation(constant.CUSTOM_FORMAT, eventReq.EndDateTime, location)
// 	if err != nil {
// 		p.API.LogError("Error parsing end time", "err", err.Error())
// 		return
// 	}

// 	// validate time
// 	if startTime.After(endTime) {
// 		if appErr := p.CreateBotDMPost(userID, fmt.Sprintf("Start time is after end time start-[%v] and end-[%v]", startTime, endTime)); appErr != nil {
// 			p.API.LogError("Error creating bot post", "apErr", appErr.Error())
// 			return
// 		}
// 		return
// 	}

// 	newEvent := calendar.Event{
// 		Summary: eventReq.EvName,
// 		Start:   &calendar.EventDateTime{DateTime: startTime.Format(time.RFC3339)},
// 		End:     &calendar.EventDateTime{DateTime: endTime.Format(time.RFC3339)},
// 		ConferenceData: &calendar.ConferenceData{
// 			CreateRequest: &calendar.CreateConferenceRequest{
// 				ConferenceSolutionKey: &calendar.ConferenceSolutionKey{
// 					Type: "hangoutsMeet",
// 				},
// 				RequestId: RandString(10),
// 			},
// 		},
// 	}
// 	calendarId := constant.PRIMARY_CALENDAR_ID
// 	createdEvent, err := cal.service.Events.Insert(calendarId, &newEvent).Do()
// 	if err != nil {
// 		p.API.LogError("Error creating event", "err", err.Error())
// 		return
// 	}

// 	appErr := p.CreateBotDMPost(userID, fmt.Sprintf("Success! Event _[%s](%s)_ on %v has been created.",
// 		createdEvent.Summary, createdEvent.HtmlLink, startTime.Format(constant.DATE_FORMAT)))
// 	if appErr != nil {
// 		p.API.LogError("Error creating post", "err", appErr.Error())
// 		return
// 	}
// }

func (p *Plugin) deleteEvent(w http.ResponseWriter, r *http.Request) {
	html := `
	<!DOCTYPE html>
	<html>
		<head>
			<script>
				window.close();
			</script>
		</head>
	</html>
	`
	userID := r.Header.Get(constant.MATTERMOST_USER_KEY)
	eventID := r.URL.Query().Get("evtid")
	cal, err := p.getCalendarService(userID)
	if err != nil {
		if err.Error() == constant.INTERNAL_ERR_USER_NOT_FOUND {
			// tell user to connect first
			if appErr := p.CreateBotDMPost(userID, constant.ERR_CONNECT_FIRST); appErr != nil {
				p.API.LogError("Error creating bot post", "apErr", appErr.Error())
				return
			}
		}

		if appErr := p.CreateBotDMPost(userID, fmt.Sprintf("Unable to delete event. Error: %s", err)); appErr != nil {
			p.API.LogError("Error creating bot post", "apErr", appErr.Error())
		}
		return
	}

	calendarID := p.getPrimaryCalendarID(userID)
	eventToBeDeleted, err := cal.service.Events.Get(calendarID, eventID).Do()
	if err != nil {
		if appErr := p.CreateBotDMPost(userID, fmt.Sprintf("Unable to delete event. Error: %s", err)); appErr != nil {
			p.API.LogError("Error creating bot post", "apErr", appErr.Error())
		}
		return
	}
	if !eventToBeDeleted.Organizer.Self {
		if appErr := p.CreateBotDMPost(userID, "You can only delete events that you have created."); appErr != nil {
			p.API.LogError("Error creating bot post", "apErr", appErr.Error())
		}
		return
	}

	err = cal.service.Events.Delete(calendarID, eventID).Do()
	if err != nil {
		if appErr := p.CreateBotDMPost(userID, fmt.Sprintf("Unable to delete event. Error: %s", err.Error())); appErr != nil {
			p.API.LogError("Error creating bot post", "apErr", appErr.Error())
		}
		return
	}

	if appErr := p.CreateBotDMPost(userID, fmt.Sprintf("Success! Event _%s_ has been deleted.", eventToBeDeleted.Summary)); appErr != nil {
		p.API.LogError("Error creating bot post", "apErr", appErr.Error())
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	fmt.Fprint(w, html)
}

func (p *Plugin) handleEventResponse(w http.ResponseWriter, r *http.Request) {
	html := `
	<!DOCTYPE html>
	<html>
		<head>
			<script>
				window.close();
			</script>
		</head>
	</html>
	`

	userID := r.Header.Get(constant.MATTERMOST_USER_KEY)
	response := r.URL.Query().Get("response")
	eventID := r.URL.Query().Get("evtid")
	cal, err := p.getCalendarService(userID)
	if err != nil {
		if err.Error() == constant.INTERNAL_ERR_USER_NOT_FOUND {
			// tell user to connect first
			if appErr := p.CreateBotDMPost(userID, constant.ERR_CONNECT_FIRST); appErr != nil {
				p.API.LogError("Error creating bot post", "apErr", appErr.Error())
				return
			}
		}

		p.CreateBotDMPost(userID, fmt.Sprintf("Unable to respond to event. Error: %s", err))
		return
	}

	calendarID := p.getPrimaryCalendarID(userID)
	eventToBeUpdated, err := cal.service.Events.Get(calendarID, eventID).Do()
	if err != nil {
		p.CreateBotDMPost(userID, fmt.Sprintf("Error! Failed to update the response of _%s_ event.", eventToBeUpdated.Summary))
		return
	}

	for idx, attendee := range eventToBeUpdated.Attendees {
		if attendee.Self {
			eventToBeUpdated.Attendees[idx].ResponseStatus = response
		}
	}

	event, err := cal.service.Events.Update(calendarID, eventID, eventToBeUpdated).Do()
	if err != nil {
		p.CreateBotDMPost(userID, fmt.Sprintf("Error! Failed to update the response of _%s_ event.", event.Summary))
	} else {
		p.CreateBotDMPost(userID, fmt.Sprintf("Success! Event _%s_ response has been updated.", event.Summary))
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	fmt.Fprint(w, html)
}

func (p *Plugin) watchCalendar(w http.ResponseWriter, r *http.Request) {
	userID := r.URL.Query().Get("userId")
	channelID := r.Header.Get("X-Goog-Channel-ID")
	resourceID := r.Header.Get("X-Goog-Resource-ID")
	state := r.Header.Get("X-Goog-Resource-State")

	watchToken, err := p.services.lookupService.Get(models.LookupsRequest{
		UserID: userID,
		Key:    constant.WATCH_TOKEN_KEY,
	})
	if err != nil {
		p.API.LogError("Error getting watch token", "err", err.Error())
		return
	}

	channelByte, err := p.services.lookupService.Get(models.LookupsRequest{
		UserID: userID,
		Key:    constant.WATCH_CHANNEL_KEY,
	})
	if err != nil {
		p.API.LogError("Error getting watch channel", "err", err.Error())
		return
	}

	jsonMap := make(map[string]interface{})
	err = json.Unmarshal([]byte(channelByte.Value), &jsonMap)
	if err != nil {
		p.API.LogError("Error unmarshalling watch channel 1", "err", err.Error())
		return
	}

	wc, err := json.Marshal(jsonMap)
	if err != nil {
		p.API.LogError("Error marshalling watch channel 2", "err", err.Error())
		return
	}
	var channel calendar.Channel
	if err := json.Unmarshal(wc, &channel); err != nil {
		p.API.LogError("Error unmarshalling watch channel 3", "err", err.Error())
		return
	}
	if state == "sync" {
		p.API.LogInfo("watchCalendar State is => Sync")
		if appErr := p.CreateBotDMPost(userID, "Google Calendar notification have syncronized with your Mattermost!"); appErr != nil {
			p.API.LogError("Error creating bot post", "apErr", appErr.Error())
			return
		}
		return
	}

	if watchToken.Value != channelID || state != "exists" {
		cal, err := p.getCalendarService(userID)
		if err != nil {
			if err.Error() == constant.INTERNAL_ERR_USER_NOT_FOUND {
				// tell user to connect first
				if appErr := p.CreateBotDMPost(userID, constant.ERR_CONNECT_FIRST); appErr != nil {
					p.API.LogError("Error creating bot post", "apErr", appErr.Error())
					return
				}
			}

			p.API.LogError("Error watching calendar", "err", err.Error())
			return
		}
		p.API.LogInfo("watchCalendar => Stop")
		if err := cal.service.Channels.Stop(&calendar.Channel{
			Id:         channelID,
			ResourceId: resourceID,
		}).Do(); err != nil {
			p.API.LogError("Error stopping channel", "err", err.Error())
		}
	}

	// if watchToken.Value == channelID && state == "exists" {

	// }

	// success case
	p.API.LogInfo("watchCalendar State is => Exists")
	if err := p.CalendarSync(userID); err != nil {
		p.API.LogError("Error syncing calendar", "err", err.Error())
	}
}

func (p *Plugin) setSettings(w http.ResponseWriter, r *http.Request) {
	var req model.SubmitDialogRequest
	if err := Decode(r.Body, &req); err != nil {
		p.API.LogError("Parser error", "err", err.Error())
		return
	}
	submissionBytes, err := json.Marshal(req.Submission)
	if err != nil {
		p.API.LogError("Parser error", "err", err.Error())
		return
	}

	var setSettingsReq SetSettingsDialog
	if err := json.Unmarshal(submissionBytes, &setSettingsReq); err != nil {
		p.API.LogError("Parser error", "err", err.Error())
		return
	}

	userID := req.UserId
	// validate
	timeNoti, err := strconv.Atoi(setSettingsReq.TimeNotiBeforeEvent)
	if err != nil {
		if err := p.CreateBotDMPost(userID, "`TimeNotiBeforeEvent is not a number`"); err != nil {
			p.API.LogError("Error creating bot post", "err", err.Error())
			return
		}
		return
	}

	// ! must between 0 - 40320
	if timeNoti < 0 || timeNoti > 40320 {
		if err := p.CreateBotDMPost(userID, "`TimeNotiBeforeEvent is not in range 0 - 40320`"); err != nil {
			p.API.LogError("Error creating bot post", "err", err.Error())
			return
		}
		return
	}

	updatedUser := models.UpdateUser{
		Setting: models.UserSettings{
			TimeNotiBeforeEvent: timeNoti,
		},
		AllowNotify: HandleAllowNotiBooltoa(setSettingsReq.AllowNotify),
	}
	if err := p.services.userService.UpdateUser(userID, updatedUser); err != nil {
		p.API.LogError("Error updating user", "err", err.Error())
		return
	}
	if err := p.CreateBotDMPost(userID, "Successfully update settings"); err != nil {
		p.API.LogError("Error creating bot post", "err", err.Error())
		return
	}
}

func (p *Plugin) disconnectCalendar(w http.ResponseWriter, r *http.Request) {
	var req model.SubmitDialogRequest
	if err := Decode(r.Body, &req); err != nil {
		p.API.LogError("Parser error", "err", err.Error())
		return
	}
	userID := req.UserId
	if err := p.stopWatch(userID); err != nil {
		http.Error(w, "failed to stop watch", http.StatusInternalServerError)
		return
	}

	// delete all user data
	if err := p.services.userService.DeleteUserData(userID); err != nil {
		p.API.LogError("Error deleting user data when disconnect", "err", err.Error())
		if err := p.CreateBotDMPost(userID, "Error disconnecting calendar"); err != nil {
			p.API.LogError("Error creating bot post", "err", err.Error())
			return
		}
		return
	}

	if err := p.CreateBotDMPost(userID, "Disconnected calendar"); err != nil {
		p.API.LogError("Error creating bot post", "err", err.Error())
		return
	}
}
