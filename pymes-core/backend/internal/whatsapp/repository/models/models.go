package models

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/datatypes"
)

type WhatsAppConnection struct {
	OrgID              uuid.UUID  `gorm:"column:org_id;primaryKey"`
	PhoneNumberID      string     `gorm:"column:phone_number_id"`
	WABAID             string     `gorm:"column:waba_id"`
	AccessTokenEncrypt string     `gorm:"column:access_token_encrypted"`
	DisplayPhoneNumber string     `gorm:"column:display_phone_number"`
	VerifiedName       string     `gorm:"column:verified_name"`
	QualityRating      string     `gorm:"column:quality_rating"`
	MessagingLimit     string     `gorm:"column:messaging_limit"`
	IsActive           bool       `gorm:"column:is_active"`
	ConnectedAt        time.Time  `gorm:"column:connected_at"`
	DisconnectedAt     *time.Time `gorm:"column:disconnected_at"`
	CreatedAt          time.Time  `gorm:"column:created_at"`
}

func (WhatsAppConnection) TableName() string { return "whatsapp_connections" }

type WhatsAppMessage struct {
	ID               uuid.UUID      `gorm:"column:id;primaryKey"`
	OrgID            uuid.UUID      `gorm:"column:org_id"`
	PhoneNumberID    string         `gorm:"column:phone_number_id"`
	Direction        string         `gorm:"column:direction"`
	WAMessageID      string         `gorm:"column:wa_message_id"`
	ToPhone          string         `gorm:"column:to_phone"`
	FromPhone        string         `gorm:"column:from_phone"`
	MessageType      string         `gorm:"column:message_type"`
	Body             string         `gorm:"column:body"`
	TemplateName     string         `gorm:"column:template_name"`
	TemplateLanguage string         `gorm:"column:template_language"`
	TemplateParams   datatypes.JSON `gorm:"column:template_params"`
	MediaURL         string         `gorm:"column:media_url"`
	MediaMimeType    string         `gorm:"column:media_mime_type"`
	MediaCaption     string         `gorm:"column:media_caption"`
	Status           string         `gorm:"column:status"`
	ErrorCode        string         `gorm:"column:error_code"`
	ErrorMessage     string         `gorm:"column:error_message"`
	PartyID          *uuid.UUID     `gorm:"column:party_id"`
	ConversationID   *uuid.UUID     `gorm:"column:conversation_id"`
	CreatedBy        string         `gorm:"column:created_by"`
	Metadata         datatypes.JSON `gorm:"column:metadata"`
	CreatedAt        time.Time      `gorm:"column:created_at"`
	UpdatedAt        time.Time      `gorm:"column:updated_at"`
}

func (WhatsAppMessage) TableName() string { return "whatsapp_messages" }

type WhatsAppConversation struct {
	ID                 uuid.UUID  `gorm:"column:id;primaryKey"`
	OrgID              uuid.UUID  `gorm:"column:org_id"`
	PartyID            uuid.UUID  `gorm:"column:party_id"`
	Phone              string     `gorm:"column:phone"`
	PartyName          string     `gorm:"column:party_name"`
	AssignedTo         string     `gorm:"column:assigned_to"`
	Status             string     `gorm:"column:status"`
	LastMessageAt      *time.Time `gorm:"column:last_message_at"`
	LastMessagePreview string     `gorm:"column:last_message_preview"`
	UnreadCount        int        `gorm:"column:unread_count"`
	CreatedAt          time.Time  `gorm:"column:created_at"`
	UpdatedAt          time.Time  `gorm:"column:updated_at"`
}

func (WhatsAppConversation) TableName() string { return "whatsapp_conversations" }

type WhatsAppTemplate struct {
	ID              uuid.UUID      `gorm:"column:id;primaryKey"`
	OrgID           uuid.UUID      `gorm:"column:org_id"`
	MetaTemplateID  string         `gorm:"column:meta_template_id"`
	Name            string         `gorm:"column:name"`
	Language        string         `gorm:"column:language"`
	Category        string         `gorm:"column:category"`
	Status          string         `gorm:"column:status"`
	HeaderType      string         `gorm:"column:header_type"`
	HeaderText      string         `gorm:"column:header_text"`
	BodyText        string         `gorm:"column:body_text"`
	FooterText      string         `gorm:"column:footer_text"`
	Buttons         datatypes.JSON `gorm:"column:buttons"`
	ExampleParams   datatypes.JSON `gorm:"column:example_params"`
	RejectionReason string         `gorm:"column:rejection_reason"`
	CreatedAt       time.Time      `gorm:"column:created_at"`
	UpdatedAt       time.Time      `gorm:"column:updated_at"`
}

func (WhatsAppTemplate) TableName() string { return "whatsapp_templates" }

type WhatsAppOptIn struct {
	ID         uuid.UUID  `gorm:"column:id;primaryKey"`
	OrgID      uuid.UUID  `gorm:"column:org_id"`
	PartyID    uuid.UUID  `gorm:"column:party_id"`
	Phone      string     `gorm:"column:phone"`
	Status     string     `gorm:"column:status"`
	Source     string     `gorm:"column:source"`
	OptedInAt  time.Time  `gorm:"column:opted_in_at"`
	OptedOutAt *time.Time `gorm:"column:opted_out_at"`
	CreatedAt  time.Time  `gorm:"column:created_at"`
}

func (WhatsAppOptIn) TableName() string { return "whatsapp_opt_ins" }

type WhatsAppCampaign struct {
	ID               uuid.UUID      `gorm:"column:id;primaryKey"`
	OrgID            uuid.UUID      `gorm:"column:org_id"`
	Name             string         `gorm:"column:name"`
	TemplateName     string         `gorm:"column:template_name"`
	TemplateLanguage string         `gorm:"column:template_language"`
	TemplateParams   datatypes.JSON `gorm:"column:template_params"`
	TagFilter        string         `gorm:"column:tag_filter"`
	Status           string         `gorm:"column:status"`
	TotalRecipients  int            `gorm:"column:total_recipients"`
	SentCount        int            `gorm:"column:sent_count"`
	DeliveredCount   int            `gorm:"column:delivered_count"`
	ReadCount        int            `gorm:"column:read_count"`
	FailedCount      int            `gorm:"column:failed_count"`
	ScheduledAt      *time.Time     `gorm:"column:scheduled_at"`
	StartedAt        *time.Time     `gorm:"column:started_at"`
	CompletedAt      *time.Time     `gorm:"column:completed_at"`
	CreatedBy        string         `gorm:"column:created_by"`
	CreatedAt        time.Time      `gorm:"column:created_at"`
	UpdatedAt        time.Time      `gorm:"column:updated_at"`
}

func (WhatsAppCampaign) TableName() string { return "whatsapp_campaigns" }

type WhatsAppCampaignRecipient struct {
	ID          uuid.UUID  `gorm:"column:id;primaryKey"`
	CampaignID  uuid.UUID  `gorm:"column:campaign_id"`
	OrgID       uuid.UUID  `gorm:"column:org_id"`
	PartyID     uuid.UUID  `gorm:"column:party_id"`
	Phone       string     `gorm:"column:phone"`
	PartyName   string     `gorm:"column:party_name"`
	Status      string     `gorm:"column:status"`
	WAMessageID string     `gorm:"column:wa_message_id"`
	ErrorMessage string    `gorm:"column:error_message"`
	SentAt      *time.Time `gorm:"column:sent_at"`
	DeliveredAt *time.Time `gorm:"column:delivered_at"`
	ReadAt      *time.Time `gorm:"column:read_at"`
	CreatedAt   time.Time  `gorm:"column:created_at"`
}

func (WhatsAppCampaignRecipient) TableName() string { return "whatsapp_campaign_recipients" }
