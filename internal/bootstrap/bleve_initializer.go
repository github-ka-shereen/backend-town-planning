package bootstrap

import (
	"context"
	"log"
	bleveRepositories "town-planning-backend/bleve/repositories"
	"town-planning-backend/config"
	users_repositories "town-planning-backend/users/repositories"

	"go.uber.org/zap"
)

func IndexBleveData(
	ctx context.Context,
	userRepo users_repositories.UserRepository,
	bleveRepo bleveRepositories.BleveRepositoryInterface,
) {

	// Delete All Indexes first
	err := bleveRepo.DeleteAllIndices(context.Background())
	if err != nil {
		log.Fatalf("Error deleting all indices: %v", err)
	}

	// Index Users
	if users, err := userRepo.GetAllUsers(); err != nil {
		config.Logger.Error("Error fetching users for Bleve indexing", zap.Error(err))
	} else if err := bleveRepo.IndexExistingUsers(users); err != nil {
		config.Logger.Error("Failed to index users into Bleve", zap.Error(err))
	}

}
