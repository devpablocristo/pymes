package domain

import cmdomain "github.com/devpablocristo/pymes/pymes-core/backend/internal/customer_messaging/domain"

type DeliveryMode = cmdomain.DeliveryMode

const (
	DeliveryModeShareLink       = cmdomain.DeliveryModeShareLink
	DeliveryModeOfficialChannel = cmdomain.DeliveryModeOfficialChannel
)

type Connection = cmdomain.Connection
type Message = cmdomain.Message
type MessageDirection = cmdomain.MessageDirection
type MessageType = cmdomain.MessageType
type MessageStatus = cmdomain.MessageStatus
type Template = cmdomain.Template
type TemplateCategory = cmdomain.TemplateCategory
type TemplateStatus = cmdomain.TemplateStatus
type TemplateButton = cmdomain.TemplateButton
type OptIn = cmdomain.OptIn
type OptInStatus = cmdomain.OptInStatus
type OptInSource = cmdomain.OptInSource
type SendTextRequest = cmdomain.SendTextRequest
type SendTemplateRequest = cmdomain.SendTemplateRequest
type SendMediaRequest = cmdomain.SendMediaRequest
type InteractiveButton = cmdomain.InteractiveButton
type SendInteractiveRequest = cmdomain.SendInteractiveRequest
type StatusUpdate = cmdomain.StatusUpdate
type MessageFilter = cmdomain.MessageFilter
type ConnectionStats = cmdomain.ConnectionStats
type ConversationStatus = cmdomain.ConversationStatus
type Conversation = cmdomain.Conversation
type CampaignStatus = cmdomain.CampaignStatus
type Campaign = cmdomain.Campaign
type RecipientStatus = cmdomain.RecipientStatus
type CampaignRecipient = cmdomain.CampaignRecipient

const (
	DirectionInbound  = cmdomain.DirectionInbound
	DirectionOutbound = cmdomain.DirectionOutbound

	TypeText        = cmdomain.TypeText
	TypeTemplate    = cmdomain.TypeTemplate
	TypeImage       = cmdomain.TypeImage
	TypeDocument    = cmdomain.TypeDocument
	TypeAudio       = cmdomain.TypeAudio
	TypeVideo       = cmdomain.TypeVideo
	TypeInteractive = cmdomain.TypeInteractive

	StatusPending   = cmdomain.StatusPending
	StatusSent      = cmdomain.StatusSent
	StatusDelivered = cmdomain.StatusDelivered
	StatusRead      = cmdomain.StatusRead
	StatusFailed    = cmdomain.StatusFailed

	CategoryUtility        = cmdomain.CategoryUtility
	CategoryMarketing      = cmdomain.CategoryMarketing
	CategoryAuthentication = cmdomain.CategoryAuthentication

	TemplateStatusDraft    = cmdomain.TemplateStatusDraft
	TemplateStatusPending  = cmdomain.TemplateStatusPending
	TemplateStatusApproved = cmdomain.TemplateStatusApproved
	TemplateStatusRejected = cmdomain.TemplateStatusRejected
	TemplateStatusPaused   = cmdomain.TemplateStatusPaused
	TemplateStatusDisabled = cmdomain.TemplateStatusDisabled

	OptInStatusOptedIn  = cmdomain.OptInStatusOptedIn
	OptInStatusOptedOut = cmdomain.OptInStatusOptedOut

	OptInSourceManual        = cmdomain.OptInSourceManual
	OptInSourceForm          = cmdomain.OptInSourceForm
	OptInSourceImport        = cmdomain.OptInSourceImport
	OptInSourceWhatsAppReply = cmdomain.OptInSourceWhatsAppReply

	ConversationOpen     = cmdomain.ConversationOpen
	ConversationResolved = cmdomain.ConversationResolved
	ConversationOnHold   = cmdomain.ConversationOnHold

	CampaignDraft     = cmdomain.CampaignDraft
	CampaignScheduled = cmdomain.CampaignScheduled
	CampaignSending   = cmdomain.CampaignSending
	CampaignCompleted = cmdomain.CampaignCompleted
	CampaignCancelled = cmdomain.CampaignCancelled

	RecipientPending   = cmdomain.RecipientPending
	RecipientSent      = cmdomain.RecipientSent
	RecipientDelivered = cmdomain.RecipientDelivered
	RecipientRead      = cmdomain.RecipientRead
	RecipientFailed    = cmdomain.RecipientFailed
)
