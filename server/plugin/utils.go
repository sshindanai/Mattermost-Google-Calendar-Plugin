package plugin

import (
	"encoding/json"
	"io"
	"math/rand"
	"sort"
	"strings"
	"time"

	"github.com/mattermost/mattermost-plugin-api/experimental/command"
	"github.com/pkg/errors"
	"github.com/sshindanai/Mattermost-Google-Calendar-Plugin/server/constant"
	"google.golang.org/api/calendar/v3"
)

func (p *Plugin) GetGoogleCalendarIcon() (string, error) {
	icon, err := command.GetIconData(p.API, "assets/icon.svg")
	if err != nil {
		p.API.LogError("failed to load icon data", "err", err.Error())
		return "", errors.Wrap(err, "failed to get icon data")
	}
	return icon, nil
}

func HandleAllowNotiAtoBoolString(allowNoti string) string {
	if allowNoti == constant.ALLOW_NOTIFY {
		return "true"
	}
	return "false"
}

func HandleBooString(val bool) string {
	if val {
		return "true"
	}
	return "false"
}

func HandleAllowNotiBooltoa(allowNoti bool) string {
	if allowNoti {
		return constant.ALLOW_NOTIFY
	}
	return constant.NOT_ALLOW_NOTIFY
}

func (p *Plugin) insertSort(data []*calendar.Event, el *calendar.Event) []*calendar.Event {
	index := sort.Search(len(data), func(i int) bool { return data[i].Start.DateTime > el.Start.DateTime })
	data = append(data, &calendar.Event{})
	copy(data[index+1:], data[index:])
	data[index] = el
	return data
}

func (p *Plugin) amIAttendingEvent(self *calendar.EventAttendee) bool {
	if self != nil && self.ResponseStatus == constant.EV_STATUS_DECLINED {
		return false
	}

	return true
}

func (p *Plugin) retrieveMyselfForEvent(event *calendar.Event) *calendar.EventAttendee {
	for _, attendee := range event.Attendees {
		if attendee.Self {
			return attendee
		}
	}
	return nil
}

func (p *Plugin) isEventDeleted(event *calendar.Event) bool {
	return event.Status == "cancelled"
}

func (p *Plugin) eventIsOld(userID string, event *calendar.Event) bool {
	userLocation, err := p.getPrimaryCalendarLocation(userID)
	if err != nil {
		p.API.LogError("Unable to get primary calendar location", "userID", userID)
		return false
	}

	now := time.Now().In(userLocation)
	tEnd, _ := time.Parse(time.RFC3339, event.End.DateTime)
	return now.After(tEnd)
}

var src = rand.NewSource(time.Now().UnixNano())

const (
	letterBytes   = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	letterIdxBits = 6                    // 6 bits to represent a letter index
	letterIdxMask = 1<<letterIdxBits - 1 // All 1-bits, as many as letterIdxBits
	letterIdxMax  = 63 / letterIdxBits   // # of letter indices fitting in 63 bits
)

func RandString(n int) string {
	sb := strings.Builder{}
	sb.Grow(n)
	// A src.Int63() generates 63 random bits, enough for letterIdxMax characters!
	for i, cache, remain := n-1, src.Int63(), letterIdxMax; i >= 0; {
		if remain == 0 {
			cache, remain = src.Int63(), letterIdxMax
		}
		if idx := int(cache & letterIdxMask); idx < len(letterBytes) {
			sb.WriteByte(letterBytes[idx])
			i--
		}
		cache >>= letterIdxBits
		remain--
	}

	return sb.String()
}

// Encode data to json and sent to io.Writer interface
func Encode(w io.Writer, data interface{}) error {
	return json.NewEncoder(w).Encode(data)
}

// Decode json from io.Reader set it to value pointer
func Decode(r io.Reader, vPrt interface{}) error {
	return json.NewDecoder(r).Decode(vPrt)
}

type CreateEventDialog struct {
	EvName        string
	StartDateTime string
	EndDateTime   string
}

type SetSettingsDialog struct {
	AllowNotify         bool
	TimeNotiBeforeEvent string
}

type DisconnectDialog struct {
	Confirmation string
}
