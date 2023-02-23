package plugin

import (
	"sync"

	"github.com/robfig/cron/v3"
	"github.com/sshindanai/Mattermost-Google-Calendar-Plugin/server/constant"
	models "github.com/sshindanai/Mattermost-Google-Calendar-Plugin/server/internal/models"
)

func (p *Plugin) notifyCronJob() error {
	cron := cron.New()
	_, err := cron.AddFunc("@every 1m", func() {
		page := 1
		limit := 100
		secret := p.getConfiguration().EncryptionSecret
		for {

			result, err := p.services.userService.List(secret, models.ListUsersOption{
				Page:        page,
				Limit:       limit,
				AllowNotify: constant.ALLOW_NOTIFY,
			})
			if err != nil {
				p.API.LogError("Error getting users", "err", err.Error())
				return
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

					p.notifyUser(user)
				}(user)
			}
			wg.Wait()
			page++
		}
	})
	if err != nil {
		p.API.LogError("Error starting cron job", "err", err)
		return err
	}
	cron.Start()
	return nil
}

func (p *Plugin) notifyUser(user models.UserDataDto) {
	if user.AllowNotify != constant.ALLOW_NOTIFY {
		return
	}
	if err := p.remindUserV2(user); err != nil {
		p.API.LogError("Error reminding user", "err", err)
	}
}
