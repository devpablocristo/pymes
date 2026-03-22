package paymentgateway

import (
	"crypto/hmac"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
)

func (u *Usecases) verifyMPSignature(headers http.Header, body []byte) bool {
	secret := strings.TrimSpace(u.mpWebhookSecret)
	if secret == "" {
		return true
	}
	raw := strings.TrimSpace(headers.Get("X-Signature"))
	if raw == "" {
		return false
	}

	hash := hmac.New(sha256.New, []byte(secret))
	hash.Write(body)
	expected := hex.EncodeToString(hash.Sum(nil))

	candidates := []string{}
	for _, chunk := range strings.Split(raw, ",") {
		piece := strings.TrimSpace(chunk)
		if piece == "" {
			continue
		}
		if strings.Contains(piece, "=") {
			parts := strings.SplitN(piece, "=", 2)
			key := strings.ToLower(strings.TrimSpace(parts[0]))
			value := strings.ToLower(strings.TrimSpace(parts[1]))
			if key == "v1" && value != "" {
				candidates = append(candidates, value)
			}
			continue
		}
		candidates = append(candidates, strings.ToLower(piece))
	}
	if len(candidates) == 0 {
		candidates = append(candidates, strings.ToLower(raw))
	}

	for _, candidate := range candidates {
		if subtle.ConstantTimeCompare([]byte(candidate), []byte(expected)) == 1 {
			return true
		}
	}
	return false
}

func (u *Usecases) signOAuthState(orgID uuid.UUID) (string, error) {
	if orgID == uuid.Nil || strings.TrimSpace(u.mpClientSecret) == "" {
		return "", ErrInvalidOAuthState
	}
	ts := strconv.FormatInt(u.now().Unix(), 10)
	payload := orgID.String() + ":" + ts
	sum := hmac.New(sha256.New, []byte(u.mpClientSecret))
	sum.Write([]byte(payload))
	sig := hex.EncodeToString(sum.Sum(nil))
	raw := payload + ":" + sig
	return base64.RawURLEncoding.EncodeToString([]byte(raw)), nil
}

func (u *Usecases) verifyOAuthState(state string) (uuid.UUID, error) {
	decoded, err := base64.RawURLEncoding.DecodeString(strings.TrimSpace(state))
	if err != nil {
		return uuid.Nil, ErrInvalidOAuthState
	}
	parts := strings.Split(string(decoded), ":")
	if len(parts) != 3 {
		return uuid.Nil, ErrInvalidOAuthState
	}
	orgID, err := uuid.Parse(parts[0])
	if err != nil {
		return uuid.Nil, ErrInvalidOAuthState
	}
	unixTS, err := strconv.ParseInt(parts[1], 10, 64)
	if err != nil {
		return uuid.Nil, ErrInvalidOAuthState
	}
	if u.now().Sub(time.Unix(unixTS, 0)) > mpOAuthStateTTL {
		return uuid.Nil, ErrInvalidOAuthState
	}

	payload := parts[0] + ":" + parts[1]
	sum := hmac.New(sha256.New, []byte(u.mpClientSecret))
	sum.Write([]byte(payload))
	expected := hex.EncodeToString(sum.Sum(nil))
	if subtle.ConstantTimeCompare([]byte(strings.ToLower(parts[2])), []byte(expected)) != 1 {
		return uuid.Nil, ErrInvalidOAuthState
	}
	return orgID, nil
}

func (u *Usecases) buildWebhookURL(path string) string {
	base := strings.TrimSpace(u.mpRedirectURI)
	if base == "" {
		return ""
	}
	parsed, err := url.Parse(base)
	if err != nil || parsed.Scheme == "" || parsed.Host == "" {
		return ""
	}
	parsed.Path = strings.TrimSpace(path)
	parsed.RawQuery = ""
	parsed.Fragment = ""
	return parsed.String()
}

func (u *Usecases) buildFrontendURL(path string) string {
	base := strings.TrimSpace(u.frontendURL)
	if base == "" {
		return ""
	}
	return strings.TrimRight(base, "/") + "/" + strings.TrimLeft(path, "/")
}

func (u *Usecases) validateMPConfig() error {
	if strings.TrimSpace(u.mpAppID) == "" ||
		strings.TrimSpace(u.mpClientSecret) == "" ||
		strings.TrimSpace(u.mpRedirectURI) == "" ||
		u.crypto == nil ||
		u.mp == nil {
		return ErrGatewayConfigMissing
	}
	return nil
}

func normalizeGatewayMode(mode string) string {
	switch strings.ToLower(strings.TrimSpace(mode)) {
	case "", "mercadopago":
		return "mercadopago"
	case "demo":
		return "demo"
	default:
		return "mercadopago"
	}
}

func parseExternalReference(in string) (uuid.UUID, string, uuid.UUID, error) {
	parts := strings.Split(strings.TrimSpace(in), ":")
	if len(parts) != 3 {
		return uuid.Nil, "", uuid.Nil, ErrInvalidReference
	}
	orgID, err := uuid.Parse(strings.TrimSpace(parts[0]))
	if err != nil {
		return uuid.Nil, "", uuid.Nil, ErrInvalidReference
	}
	refType := normalizeReferenceType(parts[1])
	if refType == "" {
		return uuid.Nil, "", uuid.Nil, ErrInvalidReference
	}
	refID, err := uuid.Parse(strings.TrimSpace(parts[2]))
	if err != nil {
		return uuid.Nil, "", uuid.Nil, ErrInvalidReference
	}
	return orgID, refType, refID, nil
}

func normalizeProvider(v string) string {
	switch strings.ToLower(strings.TrimSpace(v)) {
	case "", providerMercadoPago:
		return providerMercadoPago
	default:
		return strings.ToLower(strings.TrimSpace(v))
	}
}

func normalizeReferenceType(v string) string {
	switch strings.ToLower(strings.TrimSpace(v)) {
	case "sale":
		return "sale"
	case "quote":
		return "quote"
	default:
		return ""
	}
}

func renderTemplate(tpl string, values map[string]string) string {
	out := tpl
	for key, value := range values {
		out = strings.ReplaceAll(out, "{"+key+"}", strings.TrimSpace(value))
	}
	return strings.TrimSpace(out)
}

func buildWhatsAppURL(phone, message string) string {
	encoded := url.QueryEscape(strings.TrimSpace(message))
	normalizedPhone := normalizePhone(phone)
	if normalizedPhone != "" {
		return "https://wa.me/" + normalizedPhone + "?text=" + encoded
	}
	return "https://wa.me/?text=" + encoded
}

func normalizePhone(in string) string {
	var b strings.Builder
	for _, r := range strings.TrimSpace(in) {
		if r >= '0' && r <= '9' {
			b.WriteRune(r)
		}
	}
	return b.String()
}

func formatMoneyARS(amount float64) string {
	sign := ""
	if amount < 0 {
		sign = "-"
		amount = -amount
	}
	intPart := int64(amount)
	dec := int64((amount - float64(intPart)) * 100)
	if dec < 0 {
		dec = -dec
	}
	grouped := groupWithDot(intPart)
	return fmt.Sprintf("%s$%s,%02d", sign, grouped, dec)
}

func groupWithDot(n int64) string {
	raw := strconv.FormatInt(n, 10)
	if len(raw) <= 3 {
		return raw
	}
	var parts []string
	for len(raw) > 3 {
		parts = append([]string{raw[len(raw)-3:]}, parts...)
		raw = raw[:len(raw)-3]
	}
	if raw != "" {
		parts = append([]string{raw}, parts...)
	}
	return strings.Join(parts, ".")
}

func anyToString(v any) string {
	switch t := v.(type) {
	case string:
		return strings.TrimSpace(t)
	case float64:
		if t == float64(int64(t)) {
			return strconv.FormatInt(int64(t), 10)
		}
		return strconv.FormatFloat(t, 'f', -1, 64)
	case int64:
		return strconv.FormatInt(t, 10)
	case int:
		return strconv.Itoa(t)
	case json.Number:
		return t.String()
	default:
		return ""
	}
}
