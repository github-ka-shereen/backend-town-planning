package services

import (
	"context"
	"errors"
	"town-planning-backend/db/models"
	"town-planning-backend/users/repositories"

	"github.com/redis/go-redis/v9"
	"gorm.io/gorm"
)

type AuthPreferencesService struct {
	userRepo    repositories.UserRepository
	db          *gorm.DB
	redisClient *redis.Client
	ctx         context.Context
}

func NewAuthPreferencesService(
	userRepo repositories.UserRepository,
	db *gorm.DB,
	redisClient *redis.Client,
	ctx context.Context,
) *AuthPreferencesService {
	return &AuthPreferencesService{
		userRepo:    userRepo,
		db:          db,
		redisClient: redisClient,
		ctx:         ctx,
	}
}

func (aps *AuthPreferencesService) SetAuthMethod(userID string, method string) error {
	validMethods := map[string]bool{
		string(models.AuthMethodMagicLink):     true,
		string(models.AuthMethodPassword):      true,
		string(models.AuthMethodAuthenticator): true,
	}

	if !validMethods[method] {
		return errors.New("invalid authentication method")
	}

	user, err := aps.userRepo.GetUserByID(userID)
	if err != nil {
		return err
	}

	// Additional validation based on requirements
	if method == string(models.AuthMethodPassword) && user.Password == "" {
		return errors.New("password must be set before enabling password auth")
	}

	if method == string(models.AuthMethodAuthenticator) && user.TOTPSecret == "" {
		return errors.New("authenticator must be set up before enabling")
	}

	// Update auth method
	err = aps.db.Model(&models.User{}).
		Where("id = ?", userID).
		Update("auth_method", method).
		Error

	if err != nil {
		return err
	}

	// Invalidate any existing sessions if changing auth method
	aps.redisClient.Del(aps.ctx, "user_sessions:"+userID)

	return nil
}

func (aps *AuthPreferencesService) GetAuthMethod(userID string) (string, error) {
	user, err := aps.userRepo.GetUserByID(userID)
	if err != nil {
		return "", err
	}
	return string(user.AuthMethod), nil
}

func (aps *AuthPreferencesService) CanUsePassword(userID string) (bool, error) {
	user, err := aps.userRepo.GetUserByID(userID)
	if err != nil {
		return false, err
	}
	return user.Password != "", nil
}

func (aps *AuthPreferencesService) CanUseAuthenticator(userID string) (bool, error) {
	user, err := aps.userRepo.GetUserByID(userID)
	if err != nil {
		return false, err
	}
	return user.TOTPSecret != "", nil
}
