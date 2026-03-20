package dto

import "github.com/google/uuid"

// --- Connection ---

type ConnectionResponse struct {
	OrgID              uuid.UUID `json:"org_id"`
	PhoneNumberID      string    `json:"phone_number_id"`
	WABAID             string    `json:"waba_id"`
	DisplayPhoneNumber string    `json:"display_phone_number"`
	VerifiedName       string    `json:"verified_name"`
	QualityRating      string    `json:"quality_rating"`
	MessagingLimit     string    `json:"messaging_limit"`
	IsActive           bool      `json:"is_active"`
	ConnectedAt        string    `json:"connected_at"`
}

type ConnectRequest struct {
	PhoneNumberID      string `json:"phone_number_id" binding:"required"`
	WABAID             string `json:"waba_id" binding:"required"`
	AccessToken        string `json:"access_token" binding:"required"`
	DisplayPhoneNumber string `json:"display_phone_number"`
	VerifiedName       string `json:"verified_name"`
}

type ConnectionStatsResponse struct {
	TotalSent      int `json:"total_sent"`
	TotalReceived  int `json:"total_received"`
	TotalDelivered int `json:"total_delivered"`
	TotalRead      int `json:"total_read"`
	TotalFailed    int `json:"total_failed"`
}

// --- Messages ---

type SendTextRequest struct {
	PartyID string `json:"party_id" binding:"required"`
	Body    string `json:"body" binding:"required"`
}

type SendTemplateRequest struct {
	PartyID      string   `json:"party_id" binding:"required"`
	TemplateName string   `json:"template_name" binding:"required"`
	Language     string   `json:"language"`
	Params       []string `json:"params"`
}

type SendMediaRequest struct {
	PartyID   string `json:"party_id" binding:"required"`
	MediaType string `json:"media_type" binding:"required"`
	MediaURL  string `json:"media_url" binding:"required"`
	Caption   string `json:"caption"`
}

type SendInteractiveRequest struct {
	PartyID string              `json:"party_id" binding:"required"`
	Body    string              `json:"body" binding:"required"`
	Buttons []InteractiveButton `json:"buttons" binding:"required,min=1,max=3"`
}

type InteractiveButton struct {
	ID    string `json:"id" binding:"required"`
	Title string `json:"title" binding:"required"`
}

type MessageResponse struct {
	ID            uuid.UUID `json:"id"`
	Direction     string    `json:"direction"`
	WAMessageID   string    `json:"wa_message_id,omitempty"`
	ToPhone       string    `json:"to_phone"`
	FromPhone     string    `json:"from_phone,omitempty"`
	MessageType   string    `json:"message_type"`
	Body          string    `json:"body,omitempty"`
	TemplateName  string    `json:"template_name,omitempty"`
	MediaURL      string    `json:"media_url,omitempty"`
	MediaCaption  string    `json:"media_caption,omitempty"`
	Status        string    `json:"status"`
	ErrorCode     string    `json:"error_code,omitempty"`
	ErrorMessage  string    `json:"error_message,omitempty"`
	PartyID       string    `json:"party_id,omitempty"`
	CreatedAt     string    `json:"created_at"`
}

type MessageListResponse struct {
	Messages []MessageResponse `json:"messages"`
	Total    int               `json:"total"`
}

// --- Templates ---

type CreateTemplateRequest struct {
	Name       string           `json:"name" binding:"required"`
	Language   string           `json:"language"`
	Category   string           `json:"category" binding:"required"`
	HeaderType string           `json:"header_type"`
	HeaderText string           `json:"header_text"`
	BodyText   string           `json:"body_text" binding:"required"`
	FooterText string           `json:"footer_text"`
	Buttons    []TemplateButton `json:"buttons"`
}

type TemplateButton struct {
	Type    string `json:"type"`
	Text    string `json:"text"`
	URL     string `json:"url,omitempty"`
	Phone   string `json:"phone,omitempty"`
	Payload string `json:"payload,omitempty"`
}

type TemplateResponse struct {
	ID              uuid.UUID        `json:"id"`
	MetaTemplateID  string           `json:"meta_template_id,omitempty"`
	Name            string           `json:"name"`
	Language        string           `json:"language"`
	Category        string           `json:"category"`
	Status          string           `json:"status"`
	HeaderType      string           `json:"header_type,omitempty"`
	HeaderText      string           `json:"header_text,omitempty"`
	BodyText        string           `json:"body_text"`
	FooterText      string           `json:"footer_text,omitempty"`
	Buttons         []TemplateButton `json:"buttons,omitempty"`
	RejectionReason string           `json:"rejection_reason,omitempty"`
	CreatedAt       string           `json:"created_at"`
	UpdatedAt       string           `json:"updated_at"`
}

// --- Opt-in ---

type OptInRequest struct {
	PartyID string `json:"party_id" binding:"required"`
	Phone   string `json:"phone" binding:"required"`
	Source  string `json:"source"`
}

type OptOutRequest struct {
	PartyID string `json:"party_id" binding:"required"`
}

type OptInResponse struct {
	ID        uuid.UUID `json:"id"`
	PartyID   uuid.UUID `json:"party_id"`
	Phone     string    `json:"phone"`
	Status    string    `json:"status"`
	Source    string    `json:"source"`
	OptedInAt string    `json:"opted_in_at"`
}

type OptInListResponse struct {
	OptIns []OptInResponse `json:"opt_ins"`
	Total  int             `json:"total"`
}
