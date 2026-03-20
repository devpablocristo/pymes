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
	Metadata         datatypes.JSON `gorm:"column:metadata"`
	CreatedAt        time.Time      `gorm:"column:created_at"`
	UpdatedAt        time.Time      `gorm:"column:updated_at"`
}

func (WhatsAppMessage) TableName() string { return "whatsapp_messages" }

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
