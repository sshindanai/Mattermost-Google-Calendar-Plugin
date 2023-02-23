package repository

import (
	"errors"
	"fmt"
	"log"

	models "github.com/sshindanai/Mattermost-Google-Calendar-Plugin/server/internal/models"
	"github.com/sshindanai/Mattermost-Google-Calendar-Plugin/server/internal/models/dbmodel"
	"github.com/sshindanai/Mattermost-Google-Calendar-Plugin/server/internal/pagination"
	"gorm.io/gorm"
)

type UserRepository interface {
	Create(dbmodel.Users) (dbmodel.Users, error)
	Update(*dbmodel.Users, *dbmodel.Users) (*dbmodel.Users, error)
	TransactionUpdate(*gorm.DB, *dbmodel.Users, *dbmodel.Users) (*dbmodel.Users, error)
	FindByUserID(string) (*dbmodel.Users, error)
	List(opts models.ListUsersOption) (*pagination.Pagination, error)
	Delete(string) error
}

type userRepository struct {
	db *gorm.DB
}

func NewUserRepository(db *gorm.DB) UserRepository {
	return &userRepository{
		db: db,
	}
}

func (u *userRepository) Create(user dbmodel.Users) (dbmodel.Users, error) {
	if err := u.db.Create(&user).Error; err != nil {
		return dbmodel.Users{}, err
	}
	return user, nil
}

func (u *userRepository) Update(user *dbmodel.Users, updateData *dbmodel.Users) (*dbmodel.Users, error) {
	fmt.Printf("user: %+v\n", user)
	fmt.Printf("updateData: %+v\n", updateData)
	if updateData == nil {
		return nil, errors.New("update data is nil")
	}

	if updateData.Email != "" {
		user.Email = updateData.Email
	}
	if updateData.CalendarToken != "" {
		user.CalendarToken = updateData.CalendarToken
	}
	if updateData.Settings != "" {
		user.Settings = updateData.Settings
	}
	if updateData.AllowNotify != "" {
		user.AllowNotify = updateData.AllowNotify
	}
	if err := u.db.Save(user).Error; err != nil {
		log.Printf("error when save user: %v\n", err)
		return nil, err
	}
	return user, nil
}

func (u *userRepository) FindByUserID(id string) (*dbmodel.Users, error) {
	var user dbmodel.Users
	result := u.db.First(&user, "id = ?", id)
	if result.Error != nil {
		if result.Error == gorm.ErrRecordNotFound {
			return nil, nil
		}
		// other error
		return nil, result.Error
	}

	return &user, nil
}

func (u *userRepository) List(opts models.ListUsersOption) (*pagination.Pagination, error) {
	var users []dbmodel.Users
	paging := &pagination.Pagination{
		Limit: opts.Limit,
		Page:  opts.Page,
	}

	query := u.db.Scopes(pagination.Paginate(users, paging, u.db)).Where("tcc_state = ?", 0)
	if opts.AllowNotify != "" {
		query = query.Where("allow_notify = ?", opts.AllowNotify)
	}
	// execute
	if err := query.Find(&users).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, err
	}

	paging.Rows = users
	return paging, nil
}

func (u *userRepository) Delete(userID string) (err error) {
	// start transaction
	tx := u.db.Begin()
	defer func() {
		if err != nil {
			if err := tx.Rollback().Error; err != nil {
				log.Printf("error when rollback transaction: %v\n", err)
			}
			return
		}

		// commit
		if err := tx.Commit().Error; err != nil {
			log.Printf("error when commit transaction: %v\n", err)
		}
	}()

	// delete lookups
	if err = tx.Where("user_id = ?", userID).Delete(&dbmodel.Lookups{}).Error; err != nil {
		return err
	}

	// delete user
	if err = tx.Where("id = ?", userID).Delete(&dbmodel.Users{}).Error; err != nil {
		return err
	}
	return nil
}

func (u *userRepository) TransactionUpdate(tx *gorm.DB, user *dbmodel.Users, updateData *dbmodel.Users) (*dbmodel.Users, error) {
	fmt.Printf("user: %+v\n", user)
	fmt.Printf("updateData: %+v\n", updateData)
	if updateData == nil {
		return nil, errors.New("update data is nil")
	}

	if updateData.Email != "" {
		user.Email = updateData.Email
	}
	if updateData.CalendarToken != "" {
		user.CalendarToken = updateData.CalendarToken
	}
	if updateData.Settings != "" {
		user.Settings = updateData.Settings
	}
	if updateData.AllowNotify != "" {
		user.AllowNotify = updateData.AllowNotify
	}
	if err := tx.Save(user).Error; err != nil {
		log.Printf("error when save user: %v\n", err)
		return nil, err
	}
	return user, nil
}
