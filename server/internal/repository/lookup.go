package repository

import (
	"fmt"

	models "github.com/sshindanai/Mattermost-Google-Calendar-Plugin/server/internal/models"
	"github.com/sshindanai/Mattermost-Google-Calendar-Plugin/server/internal/models/dbmodel"
	"gorm.io/gorm"
)

type LookupRepository interface {
	Insert(dbmodel.Lookups) error
	Update(dbmodel.Lookups) error
	Get(models.LookupsRequest) (*dbmodel.Lookups, error)
	Delete(models.LookupsRequest) error
	DeleteAllForUser(string) error
	TransactionDelete(*gorm.DB, string) error
}

type lookupRepository struct {
	db *gorm.DB
}

func NewLookupRepository(db *gorm.DB) LookupRepository {
	return &lookupRepository{
		db: db,
	}
}

func (l *lookupRepository) Insert(lookup dbmodel.Lookups) error {
	// insert lookup
	res := l.db.Create(&lookup)
	if res.Error != nil {
		fmt.Printf("error inserting lookup: %v\n", res.Error)
		return res.Error
	}
	return nil
}

func (l *lookupRepository) Update(lookup dbmodel.Lookups) error {
	// update lookup value where user_id, type, key
	if err := l.db.Model(&lookup).Where("user_id = ? AND key = ?", lookup.UserID, lookup.Key).Updates(&lookup).Error; err != nil {
		return err
	}
	return nil
}

func (l *lookupRepository) Get(lookupRequest models.LookupsRequest) (*dbmodel.Lookups, error) {
	var lookupResponse dbmodel.Lookups
	if err := l.db.Model(&models.Lookups{}).Where("user_id = ? AND key = ?", lookupRequest.UserID, lookupRequest.Key).First(&lookupResponse).Error; err != nil {
		fmt.Printf("error getting lookup repo: %v\n", err)
		return nil, err
	}
	return &lookupResponse, nil
}

func (l *lookupRepository) Delete(lookup models.LookupsRequest) error {
	// delete lookup where user_id, type, key
	res := l.db.Where("user_id = ? AND key = ?", lookup.UserID, lookup.Key).Delete(&models.Lookups{})
	if res.Error != nil {
		fmt.Printf("error deleting lookup: %v\n", res.Error)
		return res.Error
	}
	return nil
}

func (l *lookupRepository) DeleteAllForUser(userID string) error {
	// delete lookup where user_id
	res := l.db.Where("user_id = ?", userID).Delete(&models.Lookups{})
	if res.Error != nil {
		fmt.Printf("error deleting lookup: %v\n", res.Error)
		return res.Error
	}
	return nil
}

func (l *lookupRepository) TransactionDelete(tx *gorm.DB, userID string) error {
	// delete lookup where user_id
	res := tx.Where("user_id = ?", userID).Delete(&models.Lookups{})
	if res.Error != nil {
		fmt.Printf("error deleting lookup: %v\n", res.Error)
		return res.Error
	}
	return nil
}
