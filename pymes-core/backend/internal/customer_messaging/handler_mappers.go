package customer_messaging

import (
	"github.com/devpablocristo/pymes/pymes-core/backend/internal/customer_messaging/domain"
	"github.com/devpablocristo/pymes/pymes-core/backend/internal/customer_messaging/handler/dto"
)

func toMessageResponse(m domain.Message) dto.MessageResponse {
	r := dto.MessageResponse{ID: m.ID, Direction: string(m.Direction), WAMessageID: m.WAMessageID, ToPhone: m.ToPhone, FromPhone: m.FromPhone, MessageType: string(m.MessageType), Body: m.Body, TemplateName: m.TemplateName, MediaURL: m.MediaURL, MediaCaption: m.MediaCaption, Status: string(m.Status), ErrorCode: m.ErrorCode, ErrorMessage: m.ErrorMessage, CreatedAt: m.CreatedAt.Format("2006-01-02T15:04:05Z07:00")}
	if m.PartyID != nil {
		r.PartyID = m.PartyID.String()
	}
	return r
}

func toTemplateResponse(t domain.Template) dto.TemplateResponse {
	buttons := make([]dto.TemplateButton, 0, len(t.Buttons))
	for _, b := range t.Buttons {
		buttons = append(buttons, dto.TemplateButton{Type: b.Type, Text: b.Text, URL: b.URL, Phone: b.Phone, Payload: b.Payload})
	}
	return dto.TemplateResponse{ID: t.ID, MetaTemplateID: t.MetaTemplateID, Name: t.Name, Language: t.Language, Category: string(t.Category), Status: string(t.Status), HeaderType: t.HeaderType, HeaderText: t.HeaderText, BodyText: t.BodyText, FooterText: t.FooterText, Buttons: buttons, RejectionReason: t.RejectionReason, CreatedAt: t.CreatedAt.Format("2006-01-02T15:04:05Z07:00"), UpdatedAt: t.UpdatedAt.Format("2006-01-02T15:04:05Z07:00")}
}

func toOptInResponse(o domain.OptIn) dto.OptInResponse {
	return dto.OptInResponse{ID: o.ID, PartyID: o.PartyID, Phone: o.Phone, Status: string(o.Status), Source: string(o.Source), OptedInAt: o.OptedInAt.Format("2006-01-02T15:04:05Z07:00")}
}

const timeFmt = "2006-01-02T15:04:05Z07:00"

func campaignToDTO(c *domain.Campaign) dto.CampaignResponse {
	r := dto.CampaignResponse{ID: c.ID, Name: c.Name, TemplateName: c.TemplateName, TemplateLanguage: c.TemplateLanguage, TemplateParams: c.TemplateParams, TagFilter: c.TagFilter, Status: string(c.Status), TotalRecipients: c.TotalRecipients, SentCount: c.SentCount, DeliveredCount: c.DeliveredCount, ReadCount: c.ReadCount, FailedCount: c.FailedCount, CreatedBy: c.CreatedBy, CreatedAt: c.CreatedAt.Format(timeFmt), UpdatedAt: c.UpdatedAt.Format(timeFmt)}
	if c.ScheduledAt != nil {
		v := c.ScheduledAt.Format(timeFmt)
		r.ScheduledAt = &v
	}
	if c.StartedAt != nil {
		v := c.StartedAt.Format(timeFmt)
		r.StartedAt = &v
	}
	if c.CompletedAt != nil {
		v := c.CompletedAt.Format(timeFmt)
		r.CompletedAt = &v
	}
	return r
}

func campaignRecipientToDTO(c *domain.CampaignRecipient) dto.CampaignRecipientResponse {
	r := dto.CampaignRecipientResponse{ID: c.ID, PartyID: c.PartyID, Phone: c.Phone, PartyName: c.PartyName, Status: string(c.Status), WAMessageID: c.WAMessageID, ErrorMessage: c.ErrorMessage}
	if c.SentAt != nil {
		v := c.SentAt.Format(timeFmt)
		r.SentAt = &v
	}
	if c.DeliveredAt != nil {
		v := c.DeliveredAt.Format(timeFmt)
		r.DeliveredAt = &v
	}
	if c.ReadAt != nil {
		v := c.ReadAt.Format(timeFmt)
		r.ReadAt = &v
	}
	return r
}
