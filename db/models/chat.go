package models

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

type ChatMessageType string

const (
	MessageTypeText   ChatMessageType = "TEXT"
	MessageTypeSystem ChatMessageType = "SYSTEM" // User joined, etc.
	MessageTypeAction ChatMessageType = "ACTION" // Issue resolved, etc.
)

type MessageStatus string

const (
	MessageStatusSent      MessageStatus = "SENT"
	MessageStatusDelivered MessageStatus = "DELIVERED"
	MessageStatusRead      MessageStatus = "READ"
)

type ChatThreadType string

const (
	ChatThreadGroup        ChatThreadType = "GROUP"         // All approval group members
	ChatThreadSpecificUser ChatThreadType = "SPECIFIC_USER" // One specific user
	ChatThreadMixed        ChatThreadType = "MIXED"         // Custom participant mix
)

type ParticipantRole string

const (
	ParticipantRoleOwner  ParticipantRole = "OWNER"
	ParticipantRoleAdmin  ParticipantRole = "ADMIN"
	ParticipantRoleMember ParticipantRole = "MEMBER"
)

// Updated models without soft delete

type ChatThread struct {
	ID            uuid.UUID `gorm:"type:uuid;primary_key;" json:"id"`
	ApplicationID uuid.UUID `gorm:"type:uuid;not null;index" json:"application_id"`
	IssueID       uuid.UUID `gorm:"type:uuid;not null;index" json:"issue_id"`

	// Thread configuration
	ThreadType  ChatThreadType `gorm:"type:varchar(30);not null" json:"thread_type"`
	Title       string         `gorm:"type:varchar(200);not null" json:"title"`
	Description *string        `gorm:"type:text" json:"description"`

	// Dynamic participation
	CreatedByUserID uuid.UUID `gorm:"type:uuid;not null;index" json:"created_by_user_id"`
	IsActive        bool      `gorm:"default:true;index" json:"is_active"`
	IsResolved      bool      `gorm:"default:false;index" json:"is_resolved"`

	// Real-time tracking - ADDED FOR WEBSOCKET FEATURES
	LastActivityAt time.Time `gorm:"autoUpdateTime;index" json:"last_activity_at"` // Track last message/activity
	UnreadCount    int       `gorm:"default:0" json:"unread_count"`                // Cache unread count for performance

	// Relationships
	Application  Application       `gorm:"foreignKey:ApplicationID" json:"application"`
	Issue        ApplicationIssue  `gorm:"foreignKey:IssueID" json:"issue"`
	CreatedBy    User              `gorm:"foreignKey:CreatedByUserID" json:"created_by"`
	Participants []ChatParticipant `gorm:"foreignKey:ThreadID" json:"participants,omitempty"`
	Messages     []ChatMessage     `gorm:"foreignKey:ThreadID" json:"messages,omitempty"`

	// Audit fields
	CreatedAt  time.Time  `gorm:"autoCreateTime" json:"created_at"`
	UpdatedAt  time.Time  `gorm:"autoUpdateTime" json:"updated_at"`
	ResolvedAt *time.Time `json:"resolved_at"`
}

type ChatParticipant struct {
	ID       uuid.UUID `gorm:"type:uuid;primary_key;" json:"id"`
	ThreadID uuid.UUID `gorm:"type:uuid;not null;index" json:"thread_id"`
	UserID   uuid.UUID `gorm:"type:uuid;not null;index" json:"user_id"`

	// Participant role
	Role      ParticipantRole `gorm:"type:varchar(20);default:'MEMBER'" json:"role"`
	IsActive  bool            `gorm:"default:true;index" json:"is_active"`
	CanInvite bool            `gorm:"default:false" json:"can_invite"`

	// Notification preferences
	MuteNotifications bool `gorm:"default:false" json:"mute_notifications"`

	// Real-time status - ADDED FOR WEBSOCKET FEATURES
	IsOnline      bool       `gorm:"default:false" json:"is_online"`                             // Track online status
	LastSeenAt    *time.Time `gorm:"index" json:"last_seen_at"`                                  // Last seen timestamp
	TypingUntil   *time.Time `json:"typing_until"`                                               // Track typing status expiration
	UnreadCount   int        `gorm:"default:0" json:"unread_count"`                              // Per-user unread count
	LastReadAt    *time.Time `json:"last_read_at"`                                               // Last read timestamp for this user
	Subscribed    bool       `gorm:"default:true" json:"subscribed"`                             // WebSocket subscription status

	// Relationships
	Thread ChatThread `gorm:"foreignKey:ThreadID" json:"thread"`
	User   User       `gorm:"foreignKey:UserID" json:"user"`

	// Audit fields
	AddedBy   string     `gorm:"not null" json:"added_by"`
	AddedAt   time.Time  `gorm:"autoCreateTime" json:"added_at"`
	RemovedAt *time.Time `json:"removed_at"`
	UpdatedAt time.Time  `gorm:"autoUpdateTime" json:"updated_at"`
}

type ChatMessage struct {
	ID       uuid.UUID `gorm:"type:uuid;primary_key;" json:"id"`
	ThreadID uuid.UUID `gorm:"type:uuid;not null;index" json:"thread_id"`
	SenderID uuid.UUID `gorm:"type:uuid;not null;index" json:"sender_id"`

	// Message content
	Content     string          `gorm:"type:text;not null" json:"content"`
	MessageType ChatMessageType `gorm:"type:varchar(20);default:'TEXT'" json:"message_type"`

	// Message status tracking
	Status MessageStatus `gorm:"type:varchar(20);default:'SENT'" json:"status"`

	// Editing and deletion
	IsEdited  bool       `gorm:"default:false" json:"is_edited"`
	EditedAt  *time.Time `json:"edited_at"`
	IsDeleted bool       `gorm:"default:false" json:"is_deleted"`
	DeletedAt *time.Time `json:"deleted_at"`

	// Reply threading
	ParentID *uuid.UUID `gorm:"type:uuid;index" json:"parent_id"`

	// Real-time delivery tracking - ENHANCED FOR WEBSOCKET FEATURES
	DeliveredAt *time.Time `json:"delivered_at"` // When message was delivered to recipients
	ReadCount   int        `gorm:"default:0" json:"read_count"` // Cache read count for performance
	StarCount   int        `gorm:"default:0" json:"star_count"` // Cache star count for performance

	// Starring/Reactions
	StarredBy []User            `gorm:"many2many:message_stars;joinForeignKey:MessageID;joinReferences:UserID" json:"starred_by,omitempty"`
	Reactions []MessageReaction `gorm:"foreignKey:MessageID" json:"reactions,omitempty"`

	// Relationships
	Thread       ChatThread       `gorm:"foreignKey:ThreadID" json:"thread"`
	Sender       User             `gorm:"foreignKey:SenderID" json:"sender"`
	Parent       *ChatMessage     `gorm:"foreignKey:ParentID" json:"parent,omitempty"`
	Attachments  []ChatAttachment `gorm:"foreignKey:MessageID" json:"attachments,omitempty"`
	ReadReceipts []ReadReceipt    `gorm:"foreignKey:MessageID" json:"read_receipts,omitempty"`

	// Audit fields
	CreatedAt time.Time `gorm:"autoCreateTime" json:"created_at"`
	UpdatedAt time.Time `gorm:"autoUpdateTime" json:"updated_at"`
}

type ReadReceipt struct {
	ID        uuid.UUID `gorm:"type:uuid;primary_key;" json:"id"`
	MessageID uuid.UUID `gorm:"type:uuid;not null;index" json:"message_id"`
	UserID    uuid.UUID `gorm:"type:uuid;not null;index" json:"user_id"`
	ReadAt    time.Time `gorm:"not null" json:"read_at"`

	// Delivery context - ADDED FOR REAL-TIME TRACKING
	DeviceID    *string `gorm:"type:varchar(100)" json:"device_id"`     // Which device read the message
	IPAddress   *string `gorm:"type:varchar(45)" json:"ip_address"`     // IP address for audit
	UserAgent   *string `gorm:"type:text" json:"user_agent"`            // User agent string
	IsRealtime  bool    `gorm:"default:true" json:"is_realtime"`        // Whether read via WebSocket

	// Relationships
	Message ChatMessage `gorm:"foreignKey:MessageID" json:"message"`
	User    User        `gorm:"foreignKey:UserID" json:"user"`
}

type ChatAttachment struct {
	ID         uuid.UUID `gorm:"type:uuid;primary_key;" json:"id"`
	MessageID  uuid.UUID `gorm:"type:uuid;not null;index" json:"message_id"`
	DocumentID uuid.UUID `gorm:"type:uuid;not null;index" json:"document_id"`

	// Relationships
	Message  ChatMessage `gorm:"foreignKey:MessageID" json:"message"`
	Document Document    `gorm:"foreignKey:DocumentID" json:"document"`

	CreatedAt time.Time `gorm:"autoCreateTime" json:"created_at"`
}

type MessageStar struct {
	ID        uuid.UUID `gorm:"type:uuid;primary_key;" json:"id"`
	MessageID uuid.UUID `gorm:"type:uuid;not null;index" json:"message_id"`
	UserID    uuid.UUID `gorm:"type:uuid;not null;index" json:"user_id"`
	CreatedAt time.Time `gorm:"autoCreateTime" json:"created_at"`

	// Star metadata - ADDED FOR CONTEXT
	StarType string `gorm:"type:varchar(20);default:'STAR'" json:"star_type"` // STAR, IMPORTANT, etc.

	// Relationships
	Message ChatMessage `gorm:"foreignKey:MessageID" json:"message"`
	User    User        `gorm:"foreignKey:UserID" json:"user"`
}

type MessageReaction struct {
	ID        uuid.UUID `gorm:"type:uuid;primary_key;" json:"id"`
	MessageID uuid.UUID `gorm:"type:uuid;not null;index" json:"message_id"`
	UserID    uuid.UUID `gorm:"type:uuid;not null;index" json:"user_id"`
	Emoji     string    `gorm:"type:varchar(10);not null" json:"emoji"`
	CreatedAt time.Time `gorm:"autoCreateTime" json:"created_at"`

	// Reaction metadata - ADDED FOR BETTER UX
	SkinTone *string `gorm:"type:varchar(10)" json:"skin_tone"` // Skin tone variation
	IsCustom bool    `gorm:"default:false" json:"is_custom"`    // Whether it's a custom emoji

	// Relationships
	Message ChatMessage `gorm:"foreignKey:MessageID" json:"message"`
	User    User        `gorm:"foreignKey:UserID" json:"user"`
}

// NEW: TypingIndicator model for real-time typing indicators
type TypingIndicator struct {
	ID        uuid.UUID `gorm:"type:uuid;primary_key;" json:"id"`
	ThreadID  uuid.UUID `gorm:"type:uuid;not null;index" json:"thread_id"`
	UserID    uuid.UUID `gorm:"type:uuid;not null;index" json:"user_id"`
	IsTyping  bool      `gorm:"not null" json:"is_typing"`
	ExpiresAt time.Time `gorm:"not null;index" json:"expires_at"` // Auto-expire typing indicators

	// Relationships
	Thread ChatThread `gorm:"foreignKey:ThreadID" json:"thread"`
	User   User       `gorm:"foreignKey:UserID" json:"user"`

	CreatedAt time.Time `gorm:"autoCreateTime" json:"created_at"`
}

// BeforeCreate hooks remain the same
func (ct *ChatThread) BeforeCreate(tx *gorm.DB) error {
	if ct.ID == uuid.Nil {
		ct.ID = uuid.New()
	}
	ct.LastActivityAt = time.Now() // Initialize last activity
	return nil
}

func (cp *ChatParticipant) BeforeCreate(tx *gorm.DB) error {
	if cp.ID == uuid.Nil {
		cp.ID = uuid.New()
	}
	return nil
}

func (cm *ChatMessage) BeforeCreate(tx *gorm.DB) error {
	if cm.ID == uuid.Nil {
		cm.ID = uuid.New()
	}
	return nil
}

func (rr *ReadReceipt) BeforeCreate(tx *gorm.DB) error {
	if rr.ID == uuid.Nil {
		rr.ID = uuid.New()
	}
	return nil
}

func (ca *ChatAttachment) BeforeCreate(tx *gorm.DB) error {
	if ca.ID == uuid.Nil {
		ca.ID = uuid.New()
	}
	return nil
}

func (ms *MessageStar) BeforeCreate(tx *gorm.DB) error {
	if ms.ID == uuid.Nil {
		ms.ID = uuid.New()
	}
	return nil
}

func (mr *MessageReaction) BeforeCreate(tx *gorm.DB) error {
	if mr.ID == uuid.Nil {
		mr.ID = uuid.New()
	}
	return nil
}

func (ti *TypingIndicator) BeforeCreate(tx *gorm.DB) error {
	if ti.ID == uuid.Nil {
		ti.ID = uuid.New()
	}
	return nil
}

// Helper methods for real-time features

// UpdateLastActivity updates the thread's last activity timestamp
func (ct *ChatThread) UpdateLastActivity() {
	ct.LastActivityAt = time.Now()
}

// IsUserTyping checks if a user is currently typing in this thread
func (cp *ChatParticipant) IsUserTyping() bool {
	if cp.TypingUntil == nil {
		return false
	}
	return time.Now().Before(*cp.TypingUntil)
}

// MarkAsTyping sets the user as typing with expiration
func (cp *ChatParticipant) MarkAsTyping(duration time.Duration) {
	expiry := time.Now().Add(duration)
	cp.TypingUntil = &expiry
}

// StopTyping clears the typing status
func (cp *ChatParticipant) StopTyping() {
	cp.TypingUntil = nil
}

// MarkAsRead updates read receipt and unread counts
func (cp *ChatParticipant) MarkAsRead() {
	now := time.Now()
	cp.LastReadAt = &now
	cp.UnreadCount = 0
}

// IncrementUnreadCount increments the unread message count
func (cp *ChatParticipant) IncrementUnreadCount() {
	cp.UnreadCount++
}

// MarkAsDelivered updates message delivery timestamp
func (cm *ChatMessage) MarkAsDelivered() {
	now := time.Now()
	cm.DeliveredAt = &now
	cm.Status = MessageStatusDelivered
}

// MarkAsRead updates message read status and increments read count
func (cm *ChatMessage) MarkAsRead() {
	cm.Status = MessageStatusRead
	cm.ReadCount++
}