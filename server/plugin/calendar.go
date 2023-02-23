package plugin

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/sshindanai/Mattermost-Google-Calendar-Plugin/server/constant"
	models "github.com/sshindanai/Mattermost-Google-Calendar-Plugin/server/internal/models"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
	"google.golang.org/api/calendar/v3"
	"google.golang.org/api/option"
)

type CalendarService struct {
	service      *calendar.Service
	userSettings models.UserSettings
	allowNotify  string
	email        string
}

// CalendarConfig will return a oauth2 Config with the field set
func (p *Plugin) CalendarConfig() *oauth2.Config {
	// config := p.API.GetConfig()
	clientID := p.getConfiguration().CalendarClientID
	clientSecret := p.getConfiguration().CalendarClientSecret
	redirectUrl := fmt.Sprintf("%s/plugins/%s/oauth/complete", p.getConfiguration().SiteUrl, manifest.ID)
	return &oauth2.Config{
		ClientID:     clientID,
		ClientSecret: clientSecret,
		Endpoint:     google.Endpoint,
		RedirectURL:  redirectUrl,
		Scopes: []string{
			constant.GOOGLE_CALENDAR_API_URL,
		},
	}
}

// getCalendarService retrieve token stored in database and then generates a google calendar service
func (p *Plugin) getCalendarService(userID string) (*CalendarService, error) {
	var token oauth2.Token

	// get calendar token from database
	secret := p.getConfiguration().EncryptionSecret
	user, err := p.services.userService.GetUserByID(userID, secret)
	if err != nil {
		p.API.LogError("Error getting user", "err", err.Error())
		return nil, err
	}
	tokenInByte := []byte(user.CalendarToken)
	if err := json.Unmarshal(tokenInByte, &token); err != nil {
		return nil, err
	}

	config := p.CalendarConfig()
	ctx := context.Background()
	tokenSource := config.TokenSource(ctx, &token)
	srv, err := calendar.NewService(ctx, option.WithTokenSource(tokenSource))
	if err != nil {
		return nil, err
	}

	return &CalendarService{
		service:      srv,
		userSettings: user.Settings,
		allowNotify:  user.AllowNotify,
		email:        user.Email,
	}, nil
}

// getCalendarServiceV2 receive user instead of userID
func (p *Plugin) getCalendarServiceV2(user models.UserDataDto) (*CalendarService, error) {
	var token oauth2.Token
	tokenInByte := []byte(user.CalendarToken)
	if err := json.Unmarshal(tokenInByte, &token); err != nil {
		return nil, err
	}

	config := p.CalendarConfig()
	ctx := context.Background()
	tokenSource := config.TokenSource(ctx, &token)
	srv, err := calendar.NewService(ctx, option.WithTokenSource(tokenSource))
	if err != nil {
		return nil, err
	}
	return &CalendarService{
		service:      srv,
		userSettings: user.Settings,
		allowNotify:  user.AllowNotify,
	}, nil
}

func (p *Plugin) getPrimaryCalendarLocation(userID string) (*time.Location, error) {
	cal, err := p.getCalendarService(userID)
	if err != nil {
		p.API.LogError("Error getting calendar service", "err", err.Error())
		return nil, err
	}
	primaryCalendar, err := cal.service.Calendars.Get(constant.PRIMARY_CALENDAR_ID).Do()
	if err != nil {
		p.API.LogError("Error getting primary calendar", "err", err.Error())
		return nil, err
	}
	timezone := primaryCalendar.TimeZone
	location, err := time.LoadLocation(timezone)
	if err != nil {
		return nil, err
	}

	return location, nil
}

// CalendarSync either does a full sync or a incremental sync. Taken from googles sample code
// To better understand whats going on here, you can read https://developers.google.com/calendar/v3/sync
func (p *Plugin) CalendarSync(userID string) error {
	cal, err := p.getCalendarService(userID)
	if err != nil {
		p.API.LogError("Error getting calendar service", "err", err.Error())
		return err
	}

	request := cal.service.Events.List(constant.PRIMARY_CALENDAR_ID)
	isIncrementalSync := false

	syncToken, err := p.services.lookupService.Get(models.LookupsRequest{
		UserID: userID,
		Key:    constant.SYNC_TOKEN_KEY,
	})
	if err != nil || syncToken == nil {
		// Perform a Full Sync
		oneMonthFromNow := time.Now().AddDate(0, 1, 0).Format(time.RFC3339)
		request.TimeMin(time.Now().Format(time.RFC3339)).TimeMax(oneMonthFromNow).SingleEvents(true)
	} else {
		// Performing a Incremental Sync
		request.SyncToken(syncToken.Value).ShowDeleted(true)
		isIncrementalSync = true
	}

	var pageToken string
	var events *calendar.Events
	var allEvents []*calendar.Event

	// syncing events
	for ok := true; ok; ok = pageToken != "" {
		request.PageToken(pageToken)
		events, err = request.Do()
		if err != nil {
			if err := p.services.lookupService.Delete(models.LookupsRequest{
				UserID: userID,
				Key:    constant.SYNC_TOKEN_KEY,
			}); err != nil {
				return err
			}
			if err := p.services.lookupService.Delete(models.LookupsRequest{
				UserID: userID,
				Key:    constant.EVENTS_KEY,
			}); err != nil {
				return err
			}
			if err := p.CalendarSync(userID); err != nil {
				p.API.LogError("Error syncing calendar", "err", err.Error())
				return err
			}
		}

		if len(events.Items) != 0 {
			allEvents = append(allEvents, events.Items...)
		}
		pageToken = events.NextPageToken
	}

	if err := p.services.lookupService.Set(models.Lookups{
		UserID: userID,
		Key:    constant.SYNC_TOKEN_KEY,
		Value:  events.NextSyncToken,
	}); err != nil {
		return err
	}

	if isIncrementalSync {
		// after incremental-sync
		if err := p.updateEventsInDatabase(userID, cal.allowNotify, allEvents); err != nil {
			p.API.LogError("Error updating events in database", "err", err.Error())
			return err
		}
		return nil
	}

	// after full-sync
	sort.Slice(allEvents, func(i, j int) bool {
		return allEvents[i].Start.DateTime < allEvents[j].Start.DateTime
	})
	allEventsJSON, err := json.Marshal(allEvents)
	if err != nil {
		return err
	}

	if err := p.services.lookupService.Set(models.Lookups{
		UserID: userID,
		Key:    constant.EVENTS_KEY,
		Value:  string(allEventsJSON),
	}); err != nil {
		return err
	}

	return nil
}

func (p *Plugin) CalendarSyncV2(user models.UserDataDto) error {
	cal, err := p.getCalendarServiceV2(user)
	if err != nil {
		p.API.LogError("Error getting calendar service", "err", err.Error())
		return err
	}

	request := cal.service.Events.List(constant.PRIMARY_CALENDAR_ID)
	isIncrementalSync := false

	syncToken, err := p.services.lookupService.Get(models.LookupsRequest{
		UserID: user.UserID,
		Key:    constant.SYNC_TOKEN_KEY,
	})
	if err != nil || syncToken == nil {
		// Perform a Full Sync
		oneMonthFromNow := time.Now().AddDate(0, 1, 0).Format(time.RFC3339)
		request.TimeMin(time.Now().Format(time.RFC3339)).TimeMax(oneMonthFromNow).SingleEvents(true)
	} else {
		// Performing a Incremental Sync
		request.SyncToken(syncToken.Value).ShowDeleted(true)
		isIncrementalSync = true
	}

	var pageToken string
	var events *calendar.Events
	var allEvents []*calendar.Event

	// collect all events
	for ok := true; ok; ok = pageToken != "" {
		request.PageToken(pageToken)
		events, err = request.Do()
		if err != nil {
			if err := p.services.lookupService.Delete(models.LookupsRequest{
				UserID: user.UserID,
				Key:    constant.SYNC_TOKEN_KEY,
			}); err != nil {
				return err
			}
			if err := p.services.lookupService.Delete(models.LookupsRequest{
				UserID: user.UserID,
				Key:    constant.EVENTS_KEY,
			}); err != nil {
				return err
			}
			if err := p.CalendarSyncV2(user); err != nil {
				p.API.LogError("Error syncing calendar", "err", err.Error())
				return err
			}
		}

		if len(events.Items) != 0 {
			allEvents = append(allEvents, events.Items...)
		}
		pageToken = events.NextPageToken
	}

	if err := p.services.lookupService.Set(models.Lookups{
		UserID: user.UserID,
		Key:    constant.SYNC_TOKEN_KEY,
		Value:  events.NextSyncToken,
	}); err != nil {
		return err
	}

	// do incremental-sync
	if isIncrementalSync {
		if err := p.updateEventsInDatabase(user.UserID, cal.allowNotify, allEvents); err != nil {
			p.API.LogError("Error updating events in database", "err", err.Error())
			return err
		}
		return nil
	}

	// do full-sync
	sort.Slice(allEvents, func(i, j int) bool {
		return allEvents[i].Start.DateTime < allEvents[j].Start.DateTime
	})
	allEventsJSON, err := json.Marshal(allEvents)
	if err != nil {
		return err
	}

	if err := p.services.lookupService.Set(models.Lookups{
		UserID: user.UserID,
		Key:    constant.EVENTS_KEY,
		Value:  string(allEventsJSON),
	}); err != nil {
		return err
	}

	return nil
}

// notify when user is invited to an event
func (p *Plugin) updateEventsInDatabase(userID string, allowNotify string, latestEvents []*calendar.Event) error {
	eventsJSON, err := p.services.lookupService.Get(models.LookupsRequest{
		UserID: userID,
		Key:    constant.EVENTS_KEY,
	})
	if err != nil {
		p.API.LogError("Error getting events from database", "err", err.Error())
		return err
	}

	jsonArr := []interface{}{}
	if err := json.Unmarshal([]byte(eventsJSON.Value), &jsonArr); err != nil {
		p.API.LogError("Error unmarshalling events", "err", err.Error(), "events", eventsJSON.Value)
		return err
	}

	ev, err := json.Marshal(jsonArr)
	if err != nil {
		return err
	}

	var events []*calendar.Event
	if err := json.Unmarshal(ev, &events); err != nil {
		p.API.LogError("Error getting events from database when unmarshal", "err", err.Error())
		return err
	}

	var textToPost string
	shouldPostMessage := true
	hasChange := false
	for _, changedEvent := range latestEvents {
		for idx, oldEvent := range events {
			// If this is a event we created, we don't want to make notifications
			if changedEvent.Creator.Self {
				shouldPostMessage = false
			}
			// If a current event in our database matches a event that has changed
			if oldEvent.Id == changedEvent.Id {
				textToPost = "**_Event Updated:_**\n"

				// If the events title has changed, we want to show the difference from the old one
				if oldEvent.Summary != changedEvent.Summary {
					hasChange = true
					textToPost += fmt.Sprintf("\n**~~[%v](%s)~~** ⟶ **[%v](%s)**\n", oldEvent.Summary, oldEvent.HtmlLink, changedEvent.Summary, changedEvent.HtmlLink)
				} else {
					textToPost += fmt.Sprintf("\n**[%v](%s)**\n", changedEvent.Summary, changedEvent.HtmlLink)
				}

				oldStartTime, _ := time.Parse(time.RFC3339, oldEvent.Start.DateTime)
				oldEndTime, _ := time.Parse(time.RFC3339, oldEvent.End.DateTime)

				changedStartTime, _ := time.Parse(time.RFC3339, changedEvent.Start.DateTime)
				changedEndTime, _ := time.Parse(time.RFC3339, changedEvent.End.DateTime)

				if !oldStartTime.Equal(changedStartTime) || !oldEndTime.Equal(changedEndTime) {
					hasChange = true
					textToPost += fmt.Sprintf("**When**: ~~%s @ %s to %s~~ ⟶ %s @ %s to %s\n", oldStartTime.Format(constant.DATE_FORMAT), oldStartTime.Format(constant.TIME_FORMAT),
						oldEndTime.Format(constant.TIME_FORMAT), changedStartTime.Format(constant.DATE_FORMAT), changedStartTime.Format(constant.TIME_FORMAT), changedEndTime.Format(constant.TIME_FORMAT))
				} else {
					textToPost += fmt.Sprintf("**When**: %s @ %s to %s\n",
						changedStartTime.Format(constant.DATE_FORMAT), changedStartTime.Format(constant.TIME_FORMAT), changedEndTime.Format(constant.TIME_FORMAT))
				}

				if oldEvent.Location != changedEvent.Location {
					hasChange = true
					textToPost += fmt.Sprintf("**Where**: ~~%s~~ ⟶ %s\n", oldEvent.Location, changedEvent.Location)
				} else if changedEvent.Location != "" {
					textToPost += fmt.Sprintf("**Where**: %s\n", changedEvent.Location)
				}

				if len(oldEvent.Attendees) != len(changedEvent.Attendees) {
					hasChange = true
					textToPost += fmt.Sprintf("**Guests**: ~~%+v (Organizer) & %v more~~ ⟶ %+v (Organizer) & %v more\n",
						oldEvent.Organizer.Email, len(oldEvent.Attendees)-1, changedEvent.Organizer.Email, len(changedEvent.Attendees)-1)
				} else if changedEvent.Attendees != nil {
					textToPost += fmt.Sprintf("**Guests**: %+v (Organizer) & %v more\n",
						changedEvent.Organizer.Email, len(changedEvent.Attendees)-1)
				}

				if oldEvent.Status != changedEvent.Status {
					hasChange = true
					textToPost += fmt.Sprintf("**Status of Event**: ~~%s~~ ⟶ %s\n", strings.Title(oldEvent.Status), strings.Title(changedEvent.Status))
				} else {
					textToPost += fmt.Sprintf("**Status of Event**: %s\n", strings.Title(changedEvent.Status))
				}

				self := p.retrieveMyselfForEvent(changedEvent)
				if self != nil && changedEvent.Status != "cancelled" {
					if self.ResponseStatus == "needsAction" {
						// config := p.API.GetConfig()
						url := fmt.Sprintf("%s/plugins/%s/handleresponse?evtid=%s&",
							p.getConfiguration().SiteUrl, manifest.ID, changedEvent.Id)
						textToPost += fmt.Sprintf("**Going?**: [Yes](%s) | [No](%s) | [Maybe](%s)\n\n",
							url+"response=accepted", url+"response=declined", url+"response=tentative")
					} else if self.ResponseStatus == "declined" {
						textToPost += "**Going?**: No\n\n"
					} else if self.ResponseStatus == "tentative" {
						textToPost += "**Going?**: Maybe\n\n"
					} else {
						textToPost += "**Going?**: Yes\n\n"
					}
				}

				// If the event was deleted, we want to remove it from our events slice in our database
				if changedEvent.Status == "cancelled" {
					events = append(events[:idx], events[idx+1:]...)
				} else {
					// Otherwise we want to replace the old event with the updated event
					events[idx] = changedEvent
				}

				break
			}

			// If we couldn't find the event in the database, it must be a new event so we append it
			// and post a your invited to a users channel
			if idx == len(events)-1 {
				if changedEvent.Status != "cancelled" {
					hasChange = true
					events = p.insertSort(events, changedEvent)
					textToPost = "**_You've been invited:_**\n"
					textToPost += p.printEventSummary(userID, changedEvent)
				}
			}
		}
	}

	// in case no current events
	if len(events) == 0 {
		for _, changedEvent := range latestEvents {
			if changedEvent.Creator.Self {
				shouldPostMessage = false
			}
			if changedEvent.Status == "cancelled" {
				continue
			}
			hasChange = true
			events = p.insertSort(events, changedEvent)
			textToPost = "**_You've been invited:_**\n"
			textToPost += p.printEventSummary(userID, changedEvent)
		}
	}

	// newEvents, err := json.Marshal(latestEvents)
	newEvents, err := json.Marshal(events)
	if err != nil {
		return err
	}

	if err := p.services.lookupService.Set(models.Lookups{
		UserID: userID,
		Key:    constant.EVENTS_KEY,
		Value:  string(newEvents),
	}); err != nil {
		return err
	}
	if textToPost != "" && hasChange && shouldPostMessage && allowNotify == constant.ALLOW_NOTIFY {
		if appErr := p.CreateBotDMPost(userID, textToPost); appErr != nil {
			return appErr
		}
	}
	return nil
}

// func (p *Plugin) setupCalendarWatch(userID string) error {
// 	cal, err := p.getCalendarService(userID)
// 	if err != nil {
// 		return err
// 	}

// 	// config := p.API.GetConfig()
// 	uuid := uuid.New().String()
// 	webSocketURL := p.getConfiguration().SiteUrl
// 	channel, err := cal.service.Events.Watch(constant.PRIMARY_CALENDAR_ID, &calendar.Channel{
// 		Address: fmt.Sprintf("%s/plugins/%s/watch?userId=%s", webSocketURL, manifest.ID, userID),
// 		Id:      uuid,
// 		Type:    "web_hook",
// 	}).Do()
// 	if err != nil {
// 		p.API.LogError("Error setting up calendar watch", "err", err.Error())
// 		return err
// 	}

// 	watchChannelJSON, err := channel.MarshalJSON()
// 	if err != nil {
// 		p.API.LogError("Error marshalling watch channel", "err", err.Error())
// 		return err
// 	}
// 	if err := p.services.lookupService.Set(models.Lookups{
// 		UserID: userID,
// 		Key:    constant.WATCH_TOKEN_KEY,
// 		Value:  uuid,
// 	}); err != nil {
// 		p.API.LogError("Error setting watch token", "err", err.Error())
// 		return err
// 	}

// 	if err := p.services.lookupService.Set(models.Lookups{
// 		UserID: userID,
// 		Key:    constant.WATCH_CHANNEL_KEY,
// 		Value:  string(watchChannelJSON),
// 	}); err != nil {
// 		p.API.LogError("Error setting watch channel", "err", err.Error())
// 		return err
// 	}
// 	return nil
// }

func (p *Plugin) setupCalendarWatchV2(user models.UserDataDto) error {
	cal, err := p.getCalendarServiceV2(user)
	if err != nil {
		return err
	}

	// config := p.API.GetConfig()
	uuid := uuid.New().String()
	webSocketURL := p.getConfiguration().SiteUrl
	channel, err := cal.service.Events.Watch(constant.PRIMARY_CALENDAR_ID, &calendar.Channel{
		Address: fmt.Sprintf("%s/plugins/%s/watch?userId=%s", webSocketURL, manifest.ID, user.UserID),
		Id:      uuid,
		Type:    "web_hook",
	}).Do()
	if err != nil {
		p.API.LogError("Error setting up calendar watch", "err", err.Error())
		return err
	}

	watchChannelJSON, err := channel.MarshalJSON()
	if err != nil {
		p.API.LogError("Error marshalling watch channel", "err", err.Error())
		return err
	}
	if err := p.services.lookupService.Set(models.Lookups{
		UserID: user.UserID,
		Key:    constant.WATCH_TOKEN_KEY,
		Value:  uuid,
	}); err != nil {
		p.API.LogError("Error setting watch token", "err", err.Error())
		return err
	}

	if err := p.services.lookupService.Set(models.Lookups{
		UserID: user.UserID,
		Key:    constant.WATCH_CHANNEL_KEY,
		Value:  string(watchChannelJSON),
	}); err != nil {
		p.API.LogError("Error setting watch channel", "err", err.Error())
		return err
	}
	return nil
}

func (p *Plugin) remindUserV2(user models.UserDataDto) error {
	cal, err := p.getCalendarServiceV2(user)
	if err != nil {
		return err
	}
	events, err := cal.service.Events.List(constant.PRIMARY_CALENDAR_ID).ShowDeleted(false).
		SingleEvents(true).TimeMin(time.Now().Format(time.RFC3339)).MaxResults(10).OrderBy("startTime").Do()
	if err != nil {
		p.API.LogError("Error getting events", "err", err)
		return err
	}

	userLocation, err := p.getPrimaryCalendarLocation(user.UserID)
	if err != nil {
		return err
	}
	for _, event := range events.Items {
		if p.eventIsOld(user.UserID, event) {
			continue
		}
		self := p.retrieveMyselfForEvent(event)
		amIAttendingEvent := (p.amIAttendingEvent(self) || event.Creator.Self)
		if !p.isEventDeleted(event) && amIAttendingEvent {
			minutes := cal.userSettings.TimeNotiBeforeEvent
			t := time.Now().In(userLocation).Add(time.Duration(minutes) * time.Minute)
			minutesLater := time.Date(t.Year(), t.Month(), t.Day(), t.Hour(), t.Minute(), 0, 0, userLocation).Format(time.RFC3339)
			if minutesLater == event.Start.DateTime {
				eventFormatted := p.printEventSummary(user.UserID, event)
				if appErr := p.CreateBotDMPost(user.UserID, fmt.Sprintf("**_%d minutes until this event:_**\n\n%s", minutes, eventFormatted)); appErr != nil {
					p.API.LogError("Unable to create bot DM post", "userID", user.UserID)
					return appErr
				}
			}
		}
	}

	return nil
}

func (p *Plugin) printEventSummary(userID string, item *calendar.Event) string {
	var text string
	// config := p.API.GetConfig()
	location, err := p.getPrimaryCalendarLocation(userID)
	if err != nil {
		p.API.LogError("Unable to get primary calendar location", "userID", userID)
		return ""
	}

	date := time.Now().In(location).Format(constant.DATE_FORMAT)
	startTime, _ := time.Parse(time.RFC3339, item.Start.DateTime)
	endTime, _ := time.Parse(time.RFC3339, item.End.DateTime)
	currentTime := time.Now().In(location).Format(constant.DATE_FORMAT)
	tomorrowTime := time.Now().AddDate(0, 0, 1).In(location).Format(constant.DATE_FORMAT)
	dateToDisplay := date
	if date == currentTime {
		dateToDisplay = "Today"
	} else if date == tomorrowTime {
		dateToDisplay = "Tomorrow"
	}

	text += fmt.Sprintf("\n**[%v](%s)**\n", item.Summary, item.HtmlLink)

	timeToDisplay := fmt.Sprintf("%v to %v", startTime.Format(constant.TIME_FORMAT), endTime.Format(constant.TIME_FORMAT))
	if startTime.Format(constant.TIME_FORMAT) == "12:00 AM UTC" && endTime.Format(constant.TIME_FORMAT) == "12:00 AM UTC" {
		timeToDisplay = "All-day"
	}
	text += fmt.Sprintf("**When**: %s @ %s\n", dateToDisplay, timeToDisplay)

	if item.Location != "" {
		text += fmt.Sprintf("**Where**: %s\n", item.Location)
	}
	if item.Location == "" && item.ConferenceData != nil {
		// text += fmt.Sprintf("**Where**: [%s](%s)\n", item.ConferenceData.ConferenceSolution.Name, item.HangoutLink)
		text += fmt.Sprintf("**Where**: %s\n", item.HangoutLink)
	}

	if item.Attendees != nil {
		text += fmt.Sprintf("**Guests**: %+v (Organizer) & %v more\n", item.Organizer.Email, len(item.Attendees)-1)
	}
	text += fmt.Sprintf("**Status of Event**: %s\n", strings.Title(item.Status))

	attendee := p.retrieveMyselfForEvent(item)
	if attendee != nil {
		if attendee.ResponseStatus == "needsAction" {
			url := fmt.Sprintf("%s/plugins/%s/handleresponse?evtid=%s&",
				p.getConfiguration().SiteUrl, manifest.ID, item.Id)
			text += fmt.Sprintf("**Going?**: [Yes](%s) | [No](%s) | [Maybe](%s)\n",
				url+"response=accepted", url+"response=declined", url+"response=tentative")
		} else if attendee.ResponseStatus == "declined" {
			text += "**Going?**: No\n"
		} else if attendee.ResponseStatus == "tentative" {
			text += "**Going?**: Maybe\n"
		} else {
			text += "**Going?**: Yes\n"
		}
	}

	if item.Organizer.Self {
		text += fmt.Sprintf("[Delete Event](%s/plugins/%s/delete?evtid=%s)\n",
			p.getConfiguration().SiteUrl, manifest.ID, item.Id)
	}

	return text
}

func (p *Plugin) getPrimaryCalendarID(userID string) string {
	cal, _ := p.getCalendarService(userID)
	primaryCalendar, _ := cal.service.Calendars.Get(constant.PRIMARY_CALENDAR_ID).Do()
	return primaryCalendar.Id
}

func (p *Plugin) stopWatch(userID string) error {
	// stop watch
	watchChannelLookup, err := p.services.lookupService.Get(models.LookupsRequest{
		UserID: userID,
		Key:    constant.WATCH_CHANNEL_KEY,
	})
	if err != nil {
		p.API.LogError("Error getting watch channel", "err", err.Error())
		return err
	}

	jsonMap := make(map[string]interface{})
	err = json.Unmarshal([]byte(watchChannelLookup.Value), &jsonMap)
	if err != nil {
		p.API.LogError("Error unmarshalling watch channel 1", "err", err.Error())
		return err
	}

	wc, err := json.Marshal(jsonMap)
	if err != nil {
		p.API.LogError("Error marshalling watch channel 2", "err", err.Error())
		return err
	}
	var channel calendar.Channel
	if err := json.Unmarshal(wc, &channel); err != nil {
		p.API.LogError("Error unmarshalling watch channel 3", "err", err.Error())
		return err
	}

	cal, err := p.getCalendarService(userID)
	if err != nil {
		if err.Error() == constant.INTERNAL_ERR_USER_NOT_FOUND {
			// tell user to connect first
			if appErr := p.CreateBotDMPost(userID, constant.ERR_CONNECT_FIRST); appErr != nil {
				p.API.LogError("Error creating bot post", "apErr", appErr.Error())
				return err
			}
		}

		p.API.LogError("Error watching calendar", "err", err.Error())
		return err
	}
	if err := cal.service.Channels.Stop(&calendar.Channel{
		Id:         channel.Id,
		ResourceId: channel.ResourceId,
	}).Do(); err != nil {
		p.API.LogError("Error stopping watch channel", "err", err.Error())
		return err
	}
	return nil
}
