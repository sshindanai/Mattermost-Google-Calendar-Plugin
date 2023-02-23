package service

import (
	"errors"
	"fmt"

	models "github.com/sshindanai/Mattermost-Google-Calendar-Plugin/server/internal/models"
	"github.com/sshindanai/Mattermost-Google-Calendar-Plugin/server/internal/models/dbmodel"
	"github.com/sshindanai/Mattermost-Google-Calendar-Plugin/server/internal/repository"
	"gorm.io/gorm"
)

type LookupService interface {
	Set(models.Lookups) error
	Get(models.LookupsRequest) (*models.LookupsResponse, error)
	Delete(models.LookupsRequest) error
	DeleteAllForUser(string) error
}

type lookupService struct {
	lookupRepository repository.LookupRepository
}

func NewLookupService(lookupRepository repository.LookupRepository) LookupService {
	return &lookupService{
		lookupRepository: lookupRepository,
	}
}

func (l *lookupService) Set(lookup models.Lookups) error {
	// check if lookup exists
	res, err := l.lookupRepository.Get(models.LookupsRequest{
		UserID: lookup.UserID,
		Key:    lookup.Key,
	})
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			// insert lookup
			if err := l.lookupRepository.Insert(dbmodel.Lookups(lookup)); err != nil {
				fmt.Printf("error getting lookup inserting: %v\n", err)
				return err
			}
			return nil
		}
	}

	// update lookup
	res.Value = lookup.Value
	if err := l.lookupRepository.Update(dbmodel.Lookups(*res)); err != nil {
		fmt.Printf("error getting lookup updating: %v\n", err)
		return err
	}
	return nil
}

func (l *lookupService) Get(lookupRequest models.LookupsRequest) (*models.LookupsResponse, error) {
	if !lookupRequest.IsValid() {
		return nil, errors.New("invalid lookup request")
	}

	lookupResponse, err := l.lookupRepository.Get(lookupRequest)
	if err != nil {
		return nil, err
	}
	return &models.LookupsResponse{
		Key:   lookupResponse.Key,
		Value: lookupResponse.Value,
	}, nil
}

func (l *lookupService) Delete(lookup models.LookupsRequest) error {
	if err := l.lookupRepository.Delete(lookup); err != nil {
		return err
	}
	return nil
}

func (l *lookupService) DeleteAllForUser(userID string) error {
	if err := l.lookupRepository.DeleteAllForUser(userID); err != nil {
		return err
	}
	return nil
}
