package paymentgateway

import (
	"context"
	"strings"

	"github.com/google/uuid"
	"github.com/skip2/go-qrcode"

	gatewaydomain "github.com/devpablocristo/pymes/pymes-core/backend/internal/paymentgateway/usecases/domain"
)

func (u *Usecases) BuildSalePaymentInfoWhatsApp(
	ctx context.Context,
	tenantID uuid.UUID,
	saleID uuid.UUID,
) (WhatsAppResult, error) {
	sale, err := u.repo.GetSaleSnapshot(ctx, tenantID, saleID)
	if err != nil {
		return WhatsAppResult{}, err
	}
	bankInfo, ok, err := u.repo.GetBankInfo(ctx, tenantID)
	if err != nil {
		return WhatsAppResult{}, err
	}
	if !ok || strings.TrimSpace(bankInfo.Alias) == "" {
		return WhatsAppResult{}, ErrBankAliasMissing
	}

	tpl := u.repo.GetWhatsAppTransferTemplate(ctx, tenantID)
	msg := renderTemplate(tpl, map[string]string{
		"bank_alias":    bankInfo.Alias,
		"bank_cbu":      bankInfo.CBU,
		"bank_holder":   bankInfo.Holder,
		"bank_name":     bankInfo.Name,
		"customer_name": sale.CustomerName,
		"number":        sale.Number,
		"total":         formatMoneyARS(sale.Total),
	})

	return WhatsAppResult{
		URL:     buildWhatsAppURL(sale.CustomerPhone, msg),
		Message: msg,
	}, nil
}

func (u *Usecases) BuildSalePaymentLinkWhatsApp(
	ctx context.Context,
	tenantID uuid.UUID,
	saleID uuid.UUID,
) (gatewaydomain.PaymentPreference, WhatsAppResult, error) {
	sale, err := u.repo.GetSaleSnapshot(ctx, tenantID, saleID)
	if err != nil {
		return gatewaydomain.PaymentPreference{}, WhatsAppResult{}, err
	}

	pref, err := u.GetOrCreatePreference(ctx, tenantID, CreatePreferenceRequest{
		ReferenceType: "sale",
		ReferenceID:   saleID,
	})
	if err != nil {
		return gatewaydomain.PaymentPreference{}, WhatsAppResult{}, err
	}

	tpl := u.repo.GetWhatsAppLinkTemplate(ctx, tenantID)
	msg := renderTemplate(tpl, map[string]string{
		"customer_name": sale.CustomerName,
		"number":        sale.Number,
		"total":         formatMoneyARS(sale.Total),
		"payment_url":   pref.PaymentURL,
	})

	return pref, WhatsAppResult{
		URL:     buildWhatsAppURL(sale.CustomerPhone, msg),
		Message: msg,
	}, nil
}

func (u *Usecases) GenerateStaticQR(ctx context.Context, tenantID uuid.UUID, size int) ([]byte, error) {
	if size <= 0 {
		size = 512
	}
	bankInfo, _, err := u.repo.GetBankInfo(ctx, tenantID)
	if err != nil {
		return nil, err
	}
	alias := strings.TrimSpace(bankInfo.Alias)
	if alias == "" {
		return nil, ErrBankAliasMissing
	}
	return qrcode.Encode(alias, qrcode.Medium, size)
}
