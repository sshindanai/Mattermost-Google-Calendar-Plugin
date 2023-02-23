package service

import (
	"encoding/json"
	"errors"
	"log"
	"time"

	"github.com/sshindanai/Mattermost-Google-Calendar-Plugin/server/constant"
	"github.com/sshindanai/Mattermost-Google-Calendar-Plugin/server/helper"
	models "github.com/sshindanai/Mattermost-Google-Calendar-Plugin/server/internal/models"
	"github.com/sshindanai/Mattermost-Google-Calendar-Plugin/server/internal/models/dbmodel"
	"github.com/sshindanai/Mattermost-Google-Calendar-Plugin/server/internal/repository"
	"gorm.io/gorm"
)

type UserService interface {
	UpsertUserToken(models.UpsertUser) (*models.UserDataDto, bool, error)
	GetUserByID(userID string, secret string) (models.UserDataDto, error)
	UpdateUserSetting(string, models.UserSettings) error
	UpdateUser(string, models.UpdateUser) error
	DeleteUserData(string) error
	List(secret string, opts models.ListUsersOption) (models.ListUsersResult, error)
}

type userService struct {
	db         *gorm.DB
	userRepo   repository.UserRepository
	lookupRepo repository.LookupRepository
}

func NewUserService(db *gorm.DB, userRepo repository.UserRepository, lookupRepo repository.LookupRepository) UserService {
	return &userService{
		db:         db,
		userRepo:   userRepo,
		lookupRepo: lookupRepo,
	}
}

func (u *userService) UpsertUserToken(user models.UpsertUser) (*models.UserDataDto, bool, error) {
	isUpdate := false
	if user.UserID == "" || user.CalendarToken == "" {
		return nil, isUpdate, errors.New("invalid user id or calendar token")
	}

	// is user exist?
	currUser, err := u.userRepo.FindByUserID(user.UserID)
	if err != nil {
		log.Println("error when find user by id", "err", err.Error())
		return nil, isUpdate, err
	}

	token, err := helper.Decrypt(user.CalendarToken, user.EncryptSecret)
	if err != nil {
		log.Println("error when decrypt calendar token", "err", err.Error())
		return nil, isUpdate, err
	}
	now := time.Now()
	if currUser == nil {
		// create new user
		userSettingStr, err := constant.DefaultUserSettings.String()
		if err != nil {
			log.Println("error when convert user settings to string", "err", err.Error())
			return nil, isUpdate, err
		}
		userToCreate := dbmodel.Users{
			ID:            user.UserID,
			Email:         user.Email,
			CalendarToken: user.CalendarToken,
			Settings:      userSettingStr,
			AllowNotify:   constant.ALLOW_NOTIFY,
			CreatedAt:     now,
			UpdatedAt:     now,
		}

		createdUser, err := u.userRepo.Create(userToCreate)
		if err != nil {
			log.Println("error when create user", "err", err)
			return nil, isUpdate, err
		}
		return &models.UserDataDto{
			UserID:        createdUser.ID,
			Email:         createdUser.Email,
			CalendarToken: token,
			Settings:      constant.DefaultUserSettings,
			AllowNotify:   createdUser.AllowNotify,
		}, isUpdate, nil
	}

	// update user
	isUpdate = true
	userToUpdate := dbmodel.Users{
		CalendarToken: user.CalendarToken,
		UpdatedAt:     now,
	}

	// update
	updatedUser, err := u.userRepo.Update(currUser, &userToUpdate)
	if err != nil {
		log.Println("error when update calendar token", "err", err.Error())
		return nil, isUpdate, err
	}
	userSetting := models.UserSettings{}
	if helper.IsJSON(updatedUser.Settings) {
		if err := json.Unmarshal([]byte(updatedUser.Settings), &userSetting); err != nil {
			log.Println("error when unmarshal user setting", "err", err.Error())
			return nil, isUpdate, err
		}
	}
	return &models.UserDataDto{
		UserID:        updatedUser.ID,
		Email:         updatedUser.Email,
		CalendarToken: token,
		Settings:      userSetting,
		AllowNotify:   updatedUser.AllowNotify,
	}, isUpdate, nil
}

func (u *userService) GetUserByID(userID string, secret string) (models.UserDataDto, error) {
	user, err := u.userRepo.FindByUserID(userID)
	if err != nil {
		return models.UserDataDto{}, err
	}
	if user == nil {
		return models.UserDataDto{}, errors.New(constant.INTERNAL_ERR_USER_NOT_FOUND)
	}
	userSetting := models.UserSettings{}
	if helper.IsJSON(user.Settings) {
		if err := json.Unmarshal([]byte(user.Settings), &userSetting); err != nil {
			log.Println("error when unmarshal user setting", "err", err.Error())
			return models.UserDataDto{}, err
		}
	}
	token, err := helper.Decrypt(user.CalendarToken, secret)
	if err != nil {
		log.Println("error when decrypt calendar token", "err", err.Error())
		return models.UserDataDto{}, err
	}
	return models.UserDataDto{
		UserID:        user.ID,
		CalendarToken: token,
		Settings:      userSetting,
		AllowNotify:   user.AllowNotify,
		Email:         user.Email,
	}, nil
}

func (u *userService) UpdateUserSetting(userID string, newUserSetting models.UserSettings) error {
	user, err := u.userRepo.FindByUserID(userID)
	if err != nil {
		return err
	}
	if user == nil {
		return errors.New(constant.INTERNAL_ERR_USER_NOT_FOUND)
	}

	userSettingString, err := newUserSetting.String()
	if err != nil {
		return err
	}
	updatedUser := dbmodel.Users{
		Settings: userSettingString,
	}
	_, err = u.userRepo.Update(user, &updatedUser)
	if err != nil {
		return err
	}
	return nil
}

func (u *userService) UpdateUser(userID string, updateData models.UpdateUser) error {
	user, err := u.userRepo.FindByUserID(userID)
	if err != nil {
		return err
	}
	if user == nil {
		return errors.New(constant.INTERNAL_ERR_USER_NOT_FOUND)
	}

	userSettingString, err := updateData.Setting.String()
	if err != nil {
		return err
	}
	updatedUser := dbmodel.Users{
		CalendarToken: updateData.CalendarToken,
		Settings:      userSettingString,
		AllowNotify:   updateData.AllowNotify,
		UpdatedAt:     time.Now(),
	}
	_, err = u.userRepo.Update(user, &updatedUser)
	if err != nil {
		return err
	}
	return nil
}

func (u *userService) DeleteUserData(userID string) error {
	user, err := u.userRepo.FindByUserID(userID)
	if err != nil {
		return err
	}
	if user == nil {
		return errors.New(constant.INTERNAL_ERR_USER_NOT_FOUND)
	}

	if err := u.userRepo.Delete(userID); err != nil {
		return err
	}

	return nil
}

func (u *userService) List(secret string, opts models.ListUsersOption) (models.ListUsersResult, error) {
	result, err := u.userRepo.List(opts)
	if err != nil || result == nil {
		return models.ListUsersResult{}, err
	}
	users, ok := result.Rows.([]dbmodel.Users)
	if !ok {
		return models.ListUsersResult{}, errors.New("error when convert interface to []dbmodel.Users")
	}

	userDataDtos := make([]models.UserDataDto, 0, len(users))
	for _, user := range users {
		setting := models.UserSettings{}
		if helper.IsJSON(user.Settings) {
			if err := json.Unmarshal([]byte(user.Settings), &setting); err != nil {
				log.Println("error when unmarshal user config", "err", err.Error())
				return models.ListUsersResult{}, err
			}
		}
		token, err := helper.Decrypt(user.CalendarToken, secret)
		if err != nil {
			log.Println("error when decrypt calendar token", "err", err.Error())
			return models.ListUsersResult{}, err
		}
		userDataDtos = append(userDataDtos, models.UserDataDto{
			UserID:        user.ID,
			CalendarToken: token,
			Settings:      setting,
			Email:         user.Email,
			AllowNotify:   user.AllowNotify,
		})
	}
	return models.ListUsersResult{
		Users:      userDataDtos,
		TotalRows:  result.TotalRows,
		TotalPages: result.TotalPages,
	}, nil
}
