// services/readreceipt_service.go
package services

import (
	"time"
	"town-planning-backend/db/models"
	"town-planning-backend/config"

	"github.com/google/uuid"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

type ReadReceiptService struct {
	db *gorm.DB
}

func NewReadReceiptService(db *gorm.DB) *ReadReceiptService {
	return &ReadReceiptService{db: db}
}

// ProcessReadReceipts is the single source of truth for read receipt logic
func (s *ReadReceiptService) ProcessReadReceipts(threadID string, userID uuid.UUID, messageIDs []string, isRealtime bool) (int, error) {
	processedCount := 0
	readAt := time.Now()

	// Use transaction for atomic operations
	err := s.db.Transaction(func(tx *gorm.DB) error {
		for _, msgID := range messageIDs {
			messageUUID, err := uuid.Parse(msgID)
			if err != nil {
				config.Logger.Warn("Invalid message ID for read receipt",
					zap.String("messageID", msgID),
					zap.String("userID", userID.String()))
				continue
			}

			// Upsert read receipt
			receipt := models.ReadReceipt{
				MessageID:  messageUUID,
				UserID:     userID,
				ReadAt:     readAt,
				IsRealtime: isRealtime,
			}

			result := tx.Where("message_id = ? AND user_id = ?", messageUUID, userID).
				Assign(map[string]interface{}{
					"read_at":     readAt,
					"is_realtime": isRealtime,
				}).
				FirstOrCreate(&receipt)

			if result.Error != nil {
				return result.Error
			}

			// Update message read count if new receipt
			if result.RowsAffected == 1 {
				if err := tx.Model(&models.ChatMessage{}).
					Where("id = ?", messageUUID).
					UpdateColumn("read_count", gorm.Expr("read_count + ?", 1)).Error; err != nil {
					return err
				}
			}

			processedCount++
		}

		// Reset participant unread count
		return tx.Model(&models.ChatParticipant{}).
			Where("thread_id = ? AND user_id = ?", threadID, userID).
			Updates(map[string]interface{}{
				"unread_count": 0,
				"last_read_at": readAt,
			}).Error
	})

	return processedCount, err
}

func (s *ReadReceiptService) GetUserByID(userID string) (*models.User, error) {
	var user models.User
	if err := s.db.Where("id = ?", userID).First(&user).Error; err != nil {
		return nil, err
	}
	return &user, nil
}
