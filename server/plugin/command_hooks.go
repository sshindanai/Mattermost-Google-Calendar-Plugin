package plugin

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/mattermost/mattermost-server/v5/model"
	"github.com/mattermost/mattermost-server/v5/plugin"
	"github.com/pkg/errors"
	"github.com/sshindanai/Mattermost-Google-Calendar-Plugin/server/constant"
	"google.golang.org/api/calendar/v3"
)

// CommandHelp - about
const CommandHelp = `* |/calendar connect| - Connect your Google Calendar with your Mattermost account

---

* |/calendar create| Create a event with a title, start date-time, end date-time, and attendees (attendees are optional) - This command will create Google Meeting automatically
	* |Title| can be any title you like for the event.
	* |Start DateTime| This is the time the event starts. It should be a date and time in the format of YYYY-MM-DD@HH:MM in 24 hour time format.
		* Example: 2022-01-01@12:00
	* |End DateTime| This is the time the event ends. It should be a date and time in the format of YYYY-MM-DD@HH:MM in 24 hour time format.
		* Example: 2022-01-01@13:00
	* |Attendees - Optional| This is a list of username or email addresses of the people you want to invite to the event. You have to type within square brackets "[ ]" and separate each username or email address with space.
		* Example: [@exampleuser-1, @exampleuser-2 example@gmail.com ...]
	**Full command Example:** => | /calendar create 'my-meeting' 2022-01-01@12:00 2022-01-01@13:00 [exampleuser-1 exampleuser-2 example@gmail.com] |

---

* |/calendar summary [date]| - Get a break down of a particular date.
	* |date| can be word 'today' or 'tmr' or specific date in YYYY-MM-DD format.
		* Example: [@exampleuser-1, @exampleuser-2, example@gmail.com ...]
	**Full command Example:** => | /calendar summary today |  | /calendar summary tmr |  | /calendar summary 2022-01-01 |

---

* |/calendar settings| - User settings, you can see and change your settings.
	* |You can select these to set configuration|
		* |Allow notifications| Allow calendar to notify you in the channel.
		* |Time notify before event| This is the time before event to notify. It must be an positive integer in minute (This will effect when you allow to notify)

---

* |/calendar next| - Get the next event of today

--- 

* |/calendar disconnect| - Disconnect Google Calendar from your Mattermost account

---
`

func (p *Plugin) ExecuteCommand(c *plugin.Context, args *model.CommandArgs) (*model.CommandResponse, *model.AppError) {
	split := strings.Fields(args.Command)
	command := split[0]
	action := ""
	config := p.API.GetConfig()
	if len(split) > 1 {
		action = split[1]
	}

	if command != constant.MAIN_CMD {
		return &model.CommandResponse{}, nil
	}

	messageToPost := ""
	switch action {
	case constant.CONNECT_CMD:
		if config.ServiceSettings.SiteURL == nil {
			p.postCommandResponse(args, "Invalid SiteURL")
			return &model.CommandResponse{}, nil
		}
		txt := fmt.Sprintf("[Click here to link your Google Calendar.](%s/plugins/%s/oauth/connect)", p.getConfiguration().SiteUrl, manifest.ID)
		p.postCommandResponse(args, txt)
		return &model.CommandResponse{}, nil
	case constant.CREATE_CMD:
		messageToPost = p.executeCommandCreate(args)
	case constant.SUMMARY_CMD:
		messageToPost = p.executeCommandSummary(args)
	case constant.HELP_CMD:
		messageToPost = p.executeCommandHelp(args)
	case constant.SETTINGS_CMD:
		messageToPost = p.executeCommandSettings(args)
	case constant.NEXT_CMD:
		messageToPost = p.executeCommandNext(args)
	case constant.DISCONNECT_CMD:
		messageToPost = p.executeCommandDisconnect(args)
	default:
		messageToPost = fmt.Sprintf("Unknown command: `%v`", action)
	}

	if messageToPost != "" {
		p.postCommandResponse(args, messageToPost)
	}

	return &model.CommandResponse{}, nil
}

func (p *Plugin) getCommand() (*model.Command, error) {
	iconData, err := p.GetGoogleCalendarIcon()
	if err != nil {
		p.API.LogError("failed to load icon data", "err", err.Error())
		return nil, errors.Wrap(err, "failed to get icon data")
	}
	return &model.Command{
		Trigger:              "calendar",
		DisplayName:          "Google Calendar",
		Description:          "Integration with Google Calendar",
		AutoComplete:         true,
		AutoCompleteDesc:     "Available commands: connect, create, next, summary, settings, disconnect, help",
		AutoCompleteHint:     "[command]",
		AutocompleteData:     getAutocompleteData(),
		AutocompleteIconData: iconData,
	}, nil
}

func getAutocompleteData() *model.AutocompleteData {
	cal := model.NewAutocompleteData("calendar", "[command]", "Available commands: connect, list, summary, create, help")

	connect := model.NewAutocompleteData("connect", "", "Connect your Google Calendar with your Mattermost account")
	cal.AddCommand(connect)

	create := model.NewAutocompleteData("create", "", "Create an event with a title, Start DateTime, End DateTime, and Attendees")
	create.AddTextArgument("Title for the event you are creating, must be surrounded by single-quotes. Example: 'my-meeting'", "[title]", "")
	create.AddTextArgument("Time the event starts in YYYY-MM-DD@HH:MM format (2022-01-01@12:00).", "[start dateTime]", "")
	create.AddTextArgument("Time the event finishes in YYYY-MM-DD@HH:MM format (2022-01-01@13:00).", "[end dateTime]", "")
	create.AddTextArgument("This is a list of username or email addresses of the people you want to invite to the event. You have to type within square brackets `[ ]` and separate each username or email address with space.\n Example: [example1 example2 example@gmail.com]", "[Attendees] - Optional", "")
	cal.AddCommand(create)

	next := model.NewAutocompleteData("next", "", "Get the next event of today")
	cal.AddCommand(next)

	summary := model.NewAutocompleteData("summary", "[date]", "Get a breakdown of a particular date")
	summary.AddTextArgument("Date can be a word [today] or [tmr] or specific date in YYYY-MM-DD format (2022-01-01)", "[today] | [tmr] | [date]", "")
	// summary.SubCommands = append(summary.SubCommands, model.NewAutocompleteData("today", "", "Today's summary"), model.NewAutocompleteData("tmr", "", "Tomorrow's summary"))
	cal.AddCommand(summary)

	settings := model.NewAutocompleteData("settings", "", "User settings, you can see and change your settings.")
	cal.AddCommand(settings)

	discon := model.NewAutocompleteData("disconnect", "", "Disconnect Google Calendar from your Mattermost account.")
	cal.AddCommand(discon)

	help := model.NewAutocompleteData("help", "", "Display usage")
	cal.AddCommand(help)
	return cal
}

func (p *Plugin) executeCommandHelp(args *model.CommandArgs) string {
	return "###### Mattermost Google Calendar Plugin - Slash Command Help\n" + strings.Replace(CommandHelp, "|", "`", -1)
}

func (p *Plugin) postCommandResponse(args *model.CommandArgs, text string) {
	post := &model.Post{
		UserId:    p.botID,
		ChannelId: args.ChannelId,
		Message:   text,
	}
	p.API.SendEphemeralPost(args.UserId, post)
}

// func (p *Plugin) executeCommandCreateWithDialog(args *model.CommandArgs) string {
// 	if err := p.ValidateCalendarConnection(args); err != nil {
// 		return ""
// 	}
// 	req := model.OpenDialogRequest{
// 		TriggerId: args.TriggerId,
// 		URL:       fmt.Sprintf("%s/plugins/%s/create", p.getConfiguration().SiteUrl, manifest.ID),
// 		Dialog: model.Dialog{
// 			CallbackId: fmt.Sprintf("create_event_cb_%s_%s", args.UserId, args.ChannelId),
// 			Title:      "Create New Event",
// 			IconURL:    "https://img.icons8.com/color/48/000000/google-calendar--v2.png",
// 			Elements: []model.DialogElement{
// 				{
// 					DisplayName: "Title",
// 					Name:        "EvName",
// 					Type:        "text",
// 					Placeholder: "Event Title",
// 					MinLength:   1,
// 				},
// 				{
// 					DisplayName: "Start DateTime",
// 					Name:        "StartDateTime",
// 					Type:        "text",
// 					Placeholder: "YYYY-MM-DD@HH:MM",
// 					MinLength:   1,
// 				},
// 				{
// 					DisplayName: "End DateTime",
// 					Name:        "EndDateTime",
// 					Type:        "text",
// 					Placeholder: "YYYY-MM-DD@HH:MM",
// 					MinLength:   1,
// 				},
// 			},
// 			SubmitLabel: "Create Event",
// 			// NotifyOnCancel: true,
// 		},
// 	}
// 	if err := p.API.OpenInteractiveDialog(req); err != nil {
// 		errorMessage := "Failed to open Interactive Dialog"
// 		p.API.LogError(errorMessage, "err", err.Error())
// 		return err.Error()
// 	}
// 	return ""
// }

func (p *Plugin) executeCommandSummary(args *model.CommandArgs) string {
	split := strings.Fields(args.Command)
	userID := args.UserId
	cal, err := p.getCalendarService(userID)
	if err != nil {
		if err.Error() == constant.INTERNAL_ERR_USER_NOT_FOUND {
			// tell user to connect first
			p.postCommandResponse(args, constant.ERR_CONNECT_FIRST)
		}

		p.API.LogError("Error execute command summary", "err", err.Error())
		return ""
	}

	p.postCommandResponse(args, "Getting your summary...")
	location, err := p.getPrimaryCalendarLocation(userID)
	if err != nil {
		return err.Error()
	}

	date := time.Now().In(location)
	dateToDisplay := "Today"
	titleToDisplay := "Today's"

	// it means, specific date is given
	if len(split) == 3 {
		switch split[2] {
		case "today":
			date = time.Now().In(location)
		case "tmr":
			date = time.Now().AddDate(0, 0, 1).In(location)
			dateToDisplay = "Tomorrow"
			titleToDisplay = "Tomorrow's"
		default:
			date, err = time.ParseInLocation(constant.CUSTOM_FORMAT_NO_TIME, split[2], location)
			if err != nil {
				return "Invalid date format, please use YYYY-MM-DD"
			}
			dateToDisplay = date.Format(constant.DATE_FORMAT)
			titleToDisplay = dateToDisplay
		}
	}

	beginOfDay := time.Date(date.Year(), date.Month(), date.Day(), 0, 0, 0, 0, location).Format(time.RFC3339)
	endOfDay := time.Date(date.Year(), date.Month(), date.Day(), 23, 59, 59, 0, location).Format(time.RFC3339)
	events, err := cal.service.Events.List(constant.PRIMARY_CALENDAR_ID).ShowDeleted(false).
		SingleEvents(true).TimeMin(beginOfDay).TimeMax(endOfDay).OrderBy("startTime").Do()

	if err != nil {
		return "Error retrieiving events"
	}

	if len(events.Items) == 0 {
		if err := p.CreateBotDMPost(userID, "It seems that you don't have any events happening."); err != nil {
			p.API.LogError("Error creating bot post", "apErr", err.Error())
			return "internal error"
		}
		return ""
	}

	text := fmt.Sprintf("#### %s Schedule:\n", titleToDisplay)
	for _, item := range events.Items {
		text += p.printEventSummary(userID, item)
	}
	if err := p.CreateBotDMPost(userID, text); err != nil {
		p.API.LogError("Error creating bot post", "apErr", err.Error())
		return "internal error"
	}
	return ""
}

func (p *Plugin) executeCommandSettings(args *model.CommandArgs) string {
	secret := p.getConfiguration().EncryptionSecret
	user, err := p.services.userService.GetUserByID(args.UserId, secret)
	if err != nil {
		if err.Error() == constant.INTERNAL_ERR_USER_NOT_FOUND {
			// tell user to connect first
			p.postCommandResponse(args, constant.ERR_CONNECT_FIRST)
		}

		p.API.LogError("Error execute command settings", "err", err.Error())
		return ""
	}

	req := model.OpenDialogRequest{
		TriggerId: args.TriggerId,
		// URL:       fmt.Sprintf("%s/plugins/%s/settings", *p.API.GetConfig().ServiceSettings.SiteURL, manifest.ID),
		URL: fmt.Sprintf("%s/plugins/%s/settings", p.getConfiguration().SiteUrl, manifest.ID),
		Dialog: model.Dialog{
			CallbackId: fmt.Sprintf("settings_cb_%s_%s", args.UserId, args.ChannelId),
			Title:      "Settings",
			IconURL:    "https://img.icons8.com/color/48/000000/google-calendar--v2.png",
			Elements: []model.DialogElement{
				{
					DisplayName: "Allow notifications",
					Name:        "AllowNotify",
					Type:        "bool",
					Placeholder: "Allow Notifications",
					Optional:    true,
					Default:     HandleAllowNotiAtoBoolString(user.AllowNotify),
				},
				{
					DisplayName: "Time notify before event",
					Name:        "TimeNotiBeforeEvent",
					Type:        "text",
					Placeholder: "",
					MinLength:   1,
					HelpText:    "This is the time before event to notify. It must be an positive integer in minute between 0 and 40320",
					Default:     strconv.Itoa(user.Settings.TimeNotiBeforeEvent),
				},
			},
			SubmitLabel: "Save",
			// NotifyOnCancel: true,
		},
	}
	if err := p.API.OpenInteractiveDialog(req); err != nil {
		errorMessage := "Failed to open Interactive Dialog"
		p.API.LogError(errorMessage, "err", err.Error())
		return err.Error()
	}
	return ""
}

func (p *Plugin) executeCommandNext(args *model.CommandArgs) string {
	cal, err := p.getCalendarService(args.UserId)
	if err != nil {
		if err.Error() == constant.INTERNAL_ERR_USER_NOT_FOUND {
			// tell user to connect first
			p.postCommandResponse(args, constant.ERR_CONNECT_FIRST)
		}

		p.API.LogError("Error execute command next", "err", err.Error())
		return ""
	}

	p.postCommandResponse(args, "Getting your next event...")
	userID := args.UserId
	location, err := p.getPrimaryCalendarLocation(userID)
	if err != nil {
		return err.Error()
	}

	date := time.Now().In(location)
	start := date.Format(time.RFC3339)
	endOfDay := time.Date(date.Year(), date.Month(), date.Day(), 23, 59, 59, 0, location).Format(time.RFC3339)
	events, err := cal.service.Events.List(constant.PRIMARY_CALENDAR_ID).ShowDeleted(false).
		SingleEvents(true).MaxResults(1).TimeMin(start).TimeMax(endOfDay).OrderBy("startTime").Do()
	if err != nil {
		return "Error retrieiving events"
	}
	if len(events.Items) == 0 {
		p.CreateBotDMPost(userID, "It seems that you don't have any events happening today.")
		return ""
	}

	text := "#### Next Event:\n"
	text += p.printEventSummary(userID, events.Items[0])
	if appErr := p.CreateBotDMPost(userID, text); appErr != nil {
		p.API.LogError("Error creating bot post", "apErr", appErr.Error())
		return appErr.Error()
	}
	return ""
}

func (p *Plugin) executeCommandDisconnect(args *model.CommandArgs) string {
	if err := p.ValidateCalendarConnection(args); err != nil {
		return ""
	}

	// confirmation
	req := model.OpenDialogRequest{
		TriggerId: args.TriggerId,
		URL:       fmt.Sprintf("%s/plugins/%s/disconnect", p.getConfiguration().SiteUrl, manifest.ID),
		Dialog: model.Dialog{
			CallbackId:  fmt.Sprintf("disconnect_cb_%s_%s", args.UserId, args.ChannelId),
			Title:       "Are you sure to disconnect?",
			IconURL:     "https://img.icons8.com/color/48/000000/google-calendar--v2.png",
			SubmitLabel: "Confirm",
		},
	}
	if err := p.API.OpenInteractiveDialog(req); err != nil {
		errorMessage := "Failed to open Interactive Dialog"
		p.API.LogError(errorMessage, "err", err.Error())
		return err.Error()
	}
	return ""
}

func (p *Plugin) executeCommandCreate(args *model.CommandArgs) string {
	userID := args.UserId
	hasErr := false
	defer func() {
		if hasErr {
			p.postCommandResponse(args, fmt.Sprintf("latest command is `%s`", args.Command))
		}
	}()

	location, err := p.getPrimaryCalendarLocation(userID)
	if err != nil {
		hasErr = true
		return err.Error()
	}
	cal, err := p.getCalendarService(userID)
	if err != nil {
		hasErr = true
		return err.Error()
	}

	split := strings.Fields(args.Command)
	if len(split) < 4 {
		return "Missing start date-time"
	}

	// /calendar => split[0]
	// command name => split[1]
	title := split[2]
	if title == "" {
		return "Missing title"
	}

	//validate single quote in summary name
	if !strings.HasPrefix(title, "'") && !strings.HasSuffix(title, "'") {
		p.postCommandResponse(args, fmt.Sprintf("your latest command => [%s]", args.Command))
		return "Title must be in single quote"
	}

	start := split[3]
	startTime, err := time.ParseInLocation(constant.CUSTOM_FORMAT, start, location)
	if err != nil {
		hasErr = true
		return fmt.Sprintf("Invalid format of start date-time: %v", err)
	}

	if len(split) < 5 {
		hasErr = true
		return "Missing end date-time"
	}
	endTime, err := time.ParseInLocation(constant.CUSTOM_FORMAT, split[4], location)
	if err != nil {
		hasErr = true
		return fmt.Sprintf("Invalid format of end date-time: %v", err)
	}

	// validate time
	if startTime.After(endTime) {
		hasErr = true
		return "Start time must be before end time"
	}

	// organizer is the first attendee
	attendees := []*calendar.EventAttendee{
		{
			Email:          cal.email,
			Organizer:      true,
			Self:           true,
			ResponseStatus: "accepted",
		},
	}

	// validate email brackets => [...]
	if len(split) >= 6 {
		str0 := split[5:]
		str1 := strings.Join(str0, " ")
		re := regexp.MustCompile(`\[(.*?)\]`)
		submatchall := re.FindAllString(str1, -1)
		var memberStr string
		for _, element := range submatchall {
			element = strings.Trim(element, "[")
			element = strings.Trim(element, "]")
			memberStr = element
		}
		members := strings.Split(memberStr, " ")
		for _, m := range members {
			member := strings.TrimSpace(m)
			if member == "" {
				continue
			}

			var email string
			// come from @mention in mattermost
			if strings.HasPrefix(member, "@") {
				username := strings.Replace(member, "@", "", -1)
				user, err := p.API.GetUserByUsername(username)
				if err != nil {
					hasErr = true
					return fmt.Sprintf("username %s not found\n", username)
				}
				email = user.Email
			} else {
				// come from email address
				rx, err := regexp.Compile(constant.EMAIL_REGEX)
				if err != nil {
					hasErr = true
					return err.Error()
				}
				if !rx.MatchString(member) {
					hasErr = true
					return fmt.Sprintf("invalid email address: %s", member)
				}
				email = member
			}
			attendees = append(attendees, &calendar.EventAttendee{
				Email: email,
			})
		}
	}
	summary := title[1 : len(title)-1]
	newEvent := calendar.Event{
		Summary:   summary,
		Start:     &calendar.EventDateTime{DateTime: startTime.Format(time.RFC3339)},
		End:       &calendar.EventDateTime{DateTime: endTime.Format(time.RFC3339)},
		Reminders: &calendar.EventReminders{UseDefault: false},
		Attendees: attendees,
		ConferenceData: &calendar.ConferenceData{
			CreateRequest: &calendar.CreateConferenceRequest{
				ConferenceSolutionKey: &calendar.ConferenceSolutionKey{
					Type: "hangoutsMeet",
				},
				RequestId: RandString(10),
			},
		},
	}
	createdEvent, err := cal.service.Events.Insert(constant.PRIMARY_CALENDAR_ID, &newEvent).ConferenceDataVersion(1).SendUpdates("all").Do()
	if err != nil {
		hasErr = true
		return fmt.Sprintf("Failed to create calendar event. Error: %v", err)
	}
	if err := p.CreateBotDMPost(args.UserId, fmt.Sprintf("Success! Event _[%s](%s)_ on %v has been created.",
		createdEvent.Summary, createdEvent.HtmlLink, startTime.Format(constant.DATE_FORMAT))); err != nil {
		p.API.LogError("Error creating bot post", "apErr", err.Error())
	}

	return ""
}

func (p *Plugin) ValidateCalendarConnection(args *model.CommandArgs) error {
	secret := p.getConfiguration().EncryptionSecret
	_, err := p.services.userService.GetUserByID(args.UserId, secret)
	if err != nil {
		if err.Error() == constant.INTERNAL_ERR_USER_NOT_FOUND {
			p.postCommandResponse(args, constant.ERR_CONNECT_FIRST)
		}

		p.API.LogError("Error execute command", "err", err.Error())
		return err
	}
	return nil
}
