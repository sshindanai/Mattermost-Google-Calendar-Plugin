package plugin

import (
	"fmt"

	"github.com/mattermost/mattermost-server/v5/model"
)

// CreateBotDMPost used to post as google calendar bot to the user directly
func (p *Plugin) CreateBotDMPost(userID, message string) *model.AppError {
	channel, err := p.API.GetDirectChannel(userID, p.botID)
	if err != nil {
		p.API.LogError("Couldn't get bot's DM channel", "user_id", userID)
		return err
	}

	post := &model.Post{
		UserId:    p.botID,
		ChannelId: channel.Id,
		Message:   fmt.Sprintf("--- \n %s", message),
	}

	if _, err := p.API.CreatePost(post); err != nil {
		p.API.LogError("Couldn't create bot post", "user_id", userID)
		return err
	}
	return nil
}
