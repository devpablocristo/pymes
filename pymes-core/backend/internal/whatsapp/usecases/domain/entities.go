package domain

import (
	"time"

	"github.com/google/uuid"
)

// Connection representa la conexión WhatsApp Business de un tenant.
type Connection struct {
	OrgID              uuid.UUID
	PhoneNumberID      string
	WABAID             string
	AccessToken        string
	DisplayPhoneNumber string
	VerifiedName       string
	QualityRating      string
	MessagingLimit     string
	IsActive           bool
	ConnectedAt        time.Time
	DisconnectedAt     *time.Time
	CreatedAt          time.Time
}

// Message representa un mensaje enviado o recibido por WhatsApp.
type Message struct {
	ID               uuid.UUID
	OrgID            uuid.UUID
	PhoneNumberID    string
	Direction        MessageDirection
	WAMessageID      string
	ToPhone          string
	FromPhone        string
	MessageType      MessageType
	Body             string
	TemplateName     string
	TemplateLanguage string
	TemplateParams   []string
	MediaURL         string
	MediaMimeType    string
	MediaCaption     string
	Status           MessageStatus
	ErrorCode        string
	ErrorMessage     string
	PartyID          *uuid.UUID
	Metadata         map[string]any
	CreatedAt        time.Time
	UpdatedAt        time.Time
}

type MessageDirection string

const (
	DirectionInbound  MessageDirection = "inbound"
	DirectionOutbound MessageDirection = "outbound"
)

type MessageType string

const (
	TypeText        MessageType = "text"
	TypeTemplate    MessageType = "template"
	TypeImage       MessageType = "image"
	TypeDocument    MessageType = "document"
	TypeAudio       MessageType = "audio"
	TypeVideo       MessageType = "video"
	TypeInteractive MessageType = "interactive"
)

type MessageStatus string

const (
	StatusPending   MessageStatus = "pending"
	StatusSent      MessageStatus = "sent"
	StatusDelivered MessageStatus = "delivered"
	StatusRead      MessageStatus = "read"
	StatusFailed    MessageStatus = "failed"
)

// Template representa un template de mensaje aprobado por Meta.
type Template struct {
	ID              uuid.UUID
	OrgID           uuid.UUID
	MetaTemplateID  string
	Name            string
	Language        string
	Category        TemplateCategory
	Status          TemplateStatus
	HeaderType      string
	HeaderText      string
	BodyText        string
	FooterText      string
	Buttons         []TemplateButton
	ExampleParams   []string
	RejectionReason string
	CreatedAt       time.Time
	UpdatedAt       time.Time
}

type TemplateCategory string

const (
	CategoryUtility        TemplateCategory = "UTILITY"
	CategoryMarketing      TemplateCategory = "MARKETING"
	CategoryAuthentication TemplateCategory = "AUTHENTICATION"
)

type TemplateStatus string

const (
	TemplateStatusDraft    TemplateStatus = "draft"
	TemplateStatusPending  TemplateStatus = "pending"
	TemplateStatusApproved TemplateStatus = "approved"
	TemplateStatusRejected TemplateStatus = "rejected"
	TemplateStatusPaused   TemplateStatus = "paused"
	TemplateStatusDisabled TemplateStatus = "disabled"
)

type TemplateButton struct {
	Type    string `json:"type"`
	Text    string `json:"text"`
	URL     string `json:"url,omitempty"`
	Phone   string `json:"phone,omitempty"`
	Payload string `json:"payload,omitempty"`
}

// OptIn representa el consentimiento de un contacto para recibir mensajes.
type OptIn struct {
	ID         uuid.UUID
	OrgID      uuid.UUID
	PartyID    uuid.UUID
	Phone      string
	Status     OptInStatus
	Source     OptInSource
	OptedInAt  time.Time
	OptedOutAt *time.Time
	CreatedAt  time.Time
}

type OptInStatus string

const (
	OptInStatusOptedIn  OptInStatus = "opted_in"
	OptInStatusOptedOut OptInStatus = "opted_out"
)

type OptInSource string

const (
	OptInSourceManual        OptInSource = "manual"
	OptInSourceForm          OptInSource = "form"
	OptInSourceImport        OptInSource = "import"
	OptInSourceWhatsAppReply OptInSource = "whatsapp_reply"
)

// SendTextRequest es el input para enviar un mensaje de texto directo.
type SendTextRequest struct {
	OrgID   uuid.UUID
	PartyID uuid.UUID
	Body    string
	Actor   string
}

// SendTemplateRequest es el input para enviar un template message.
type SendTemplateRequest struct {
	OrgID        uuid.UUID
	PartyID      uuid.UUID
	TemplateName string
	Language     string
	Params       []string
	Actor        string
}

// SendMediaRequest es el input para enviar un mensaje con media.
type SendMediaRequest struct {
	OrgID     uuid.UUID
	PartyID   uuid.UUID
	MediaType MessageType
	MediaURL  string
	Caption   string
	Actor     string
}

// InteractiveButton para mensajes interactivos.
type InteractiveButton struct {
	ID    string `json:"id"`
	Title string `json:"title"`
}

// SendInteractiveRequest es el input para enviar mensaje con botones.
type SendInteractiveRequest struct {
	OrgID   uuid.UUID
	PartyID uuid.UUID
	Body    string
	Buttons []InteractiveButton
	Actor   string
}

// StatusUpdate representa una actualización de estado desde Meta webhook.
type StatusUpdate struct {
	WAMessageID string
	Status      MessageStatus
	Timestamp   time.Time
	ErrorCode   string
	ErrorTitle  string
}

// MessageFilter para listar mensajes con filtros.
type MessageFilter struct {
	OrgID     uuid.UUID
	PartyID   *uuid.UUID
	Direction *MessageDirection
	Status    *MessageStatus
	Limit     int
	Offset    int
}

// ConnectionStats estadísticas de la conexión WhatsApp.
type ConnectionStats struct {
	TotalSent      int `json:"total_sent"`
	TotalReceived  int `json:"total_received"`
	TotalDelivered int `json:"total_delivered"`
	TotalRead      int `json:"total_read"`
	TotalFailed    int `json:"total_failed"`
}
