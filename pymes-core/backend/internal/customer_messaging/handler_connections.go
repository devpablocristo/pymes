package customer_messaging

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/devpablocristo/pymes/pymes-core/backend/internal/customer_messaging/handler/dto"
	"github.com/devpablocristo/pymes/pymes-core/backend/internal/shared/handlers"
	httperrors "github.com/devpablocristo/pymes/pymes-core/shared/backend/httperrors"
)

func (h *Handler) GetConnection(c *gin.Context) {
	tenantID, ok := handlers.ParseAuthTenantID(c)
	if !ok {
		return
	}
	conn, err := h.uc.GetConnection(c.Request.Context(), tenantID)
	if err != nil {
		httperrors.Respond(c, err)
		return
	}
	c.JSON(http.StatusOK, dto.ConnectionResponse{TenantID: conn.TenantID, PhoneNumberID: conn.PhoneNumberID, WABAID: conn.WABAID, DisplayPhoneNumber: conn.DisplayPhoneNumber, VerifiedName: conn.VerifiedName, QualityRating: conn.QualityRating, MessagingLimit: conn.MessagingLimit, IsActive: conn.IsActive, ConnectedAt: conn.ConnectedAt.Format("2006-01-02T15:04:05Z07:00")})
}

func (h *Handler) Connect(c *gin.Context) {
	tenantID, ok := handlers.ParseAuthTenantID(c)
	if !ok {
		return
	}
	var body dto.ConnectRequest
	if err := c.ShouldBindJSON(&body); err != nil {
		writeBadRequest(c, "invalid request body")
		return
	}
	conn, err := h.uc.Connect(c.Request.Context(), tenantID, body.PhoneNumberID, body.WABAID, body.AccessToken, body.DisplayPhoneNumber, body.VerifiedName)
	if err != nil {
		httperrors.Respond(c, err)
		return
	}
	c.JSON(http.StatusCreated, dto.ConnectionResponse{TenantID: conn.TenantID, PhoneNumberID: conn.PhoneNumberID, WABAID: conn.WABAID, DisplayPhoneNumber: conn.DisplayPhoneNumber, VerifiedName: conn.VerifiedName, QualityRating: conn.QualityRating, MessagingLimit: conn.MessagingLimit, IsActive: conn.IsActive, ConnectedAt: conn.ConnectedAt.Format("2006-01-02T15:04:05Z07:00")})
}

func (h *Handler) Disconnect(c *gin.Context) {
	tenantID, ok := handlers.ParseAuthTenantID(c)
	if !ok {
		return
	}
	if err := h.uc.Disconnect(c.Request.Context(), tenantID); err != nil {
		httperrors.Respond(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}

func (h *Handler) GetConnectionStats(c *gin.Context) {
	tenantID, ok := handlers.ParseAuthTenantID(c)
	if !ok {
		return
	}
	stats, err := h.uc.GetConnectionStats(c.Request.Context(), tenantID)
	if err != nil {
		httperrors.Respond(c, err)
		return
	}
	c.JSON(http.StatusOK, dto.ConnectionStatsResponse{TotalSent: stats.TotalSent, TotalReceived: stats.TotalReceived, TotalDelivered: stats.TotalDelivered, TotalRead: stats.TotalRead, TotalFailed: stats.TotalFailed})
}
