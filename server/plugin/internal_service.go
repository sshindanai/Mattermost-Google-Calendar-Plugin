package plugin

import (
	"fmt"
	"log"

	"github.com/sshindanai/Mattermost-Google-Calendar-Plugin/server/internal/repository"
	"github.com/sshindanai/Mattermost-Google-Calendar-Plugin/server/internal/service"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

type InternalService struct {
	db            *gorm.DB
	userService   service.UserService
	lookupService service.LookupService
	stateService  service.ConnectStateService
}

func (p *Plugin) ConnectDB() (*gorm.DB, error) {
	host := p.getConfiguration().DbHost
	user := p.getConfiguration().DbUser
	password := p.getConfiguration().DbPassword
	name := p.getConfiguration().DbName
	port := p.getConfiguration().DbPort

	dsn := fmt.Sprintf("host=%s user=%s password=%s dbname=%s port=%s sslmode=disable", host, user, password, name, port)
	// dsn := "host=localhost user=mmuser password=mmuser_password dbname=postgres port=5432 sslmode=disable"
	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{})
	if err != nil {
		p.API.LogError("failed to connect database", "err=>", err.Error())
		return nil, err
	}
	sqlDB, err := db.DB()
	if err != nil {
		p.API.LogError("failed to connect database", "err=>", err.Error())
		return nil, err
	}

	if err := sqlDB.Ping(); err != nil {
		p.API.LogError("failed to ping database", "err=>", err.Error())
		return nil, err
	}

	p.API.LogInfo("connected to database successfully!")
	return db, nil
}

func (p *Plugin) CloseDb() error {
	p.API.LogInfo("closing database connection")
	db, err := p.services.db.DB()
	if err != nil {
		return err
	}
	if err := db.Close(); err != nil {
		return err
	}
	return nil
}

func (p *Plugin) NewInternalService() *InternalService {
	// check required config variable
	if err := p.checkEnv(); err != nil {
		p.API.LogError("failed to check sanity", "err", err)
		log.Fatal(err)
	}

	// set db connection
	db, err := p.ConnectDB()
	if err != nil {
		log.Fatalf("failed to connect database: %v", err.Error())
	}

	// state
	stateRepo := repository.NewConnectStateRepository(db)
	stateService := service.NewConnectStateService(stateRepo)

	// lookup
	lookupRepo := repository.NewLookupRepository(db)
	lookupService := service.NewLookupService(lookupRepo)

	// user
	userRepo := repository.NewUserRepository(db)
	userService := service.NewUserService(db, userRepo, lookupRepo)
	return &InternalService{
		db:            db,
		userService:   userService,
		lookupService: lookupService,
		stateService:  stateService,
	}
}

func (p *Plugin) checkEnv() error {
	// check required config variable
	if p.getConfiguration().SiteUrl == "" {
		return fmt.Errorf("environment variable Site Url is not set")
	}
	if p.getConfiguration().DbHost == "" {
		return fmt.Errorf("environment variable DB_HOST is not set")
	}
	if p.getConfiguration().DbUser == "" {
		return fmt.Errorf("environment variable DB_USER is not set")
	}
	if p.getConfiguration().DbPassword == "" {
		return fmt.Errorf("environment variable DB_PASSWORD is not set")
	}
	if p.getConfiguration().DbName == "" {
		return fmt.Errorf("environment variable DB_NAME is not set")
	}
	if p.getConfiguration().DbPort == "" {
		return fmt.Errorf("environment variable DB_PORT is not set")
	}
	if p.getConfiguration().CalendarClientID == "" {
		return fmt.Errorf("environment variable CALENDAR_CLIENT_ID is not set")
	}
	if p.getConfiguration().CalendarClientSecret == "" {
		return fmt.Errorf("environment variable CALENDAR_CLIENT_SECRET is not set")
	}
	if p.getConfiguration().EncryptionSecret == "" {
		return fmt.Errorf("environment variable ENCRYPTION_SECRET is not set")
	}
	return nil
}
