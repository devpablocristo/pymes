package axiscallbacks

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"io"
	"net/http"
	"strings"
	"time"

	ginmw "github.com/devpablocristo/platform/http/gin/go"
	"github.com/gin-gonic/gin"
)

const defaultCallbackMaxSkew = 5 * time.Minute

type Config struct {
	Token   string
	MaxSkew time.Duration
	Now     func() time.Time
}

func NewNexusCallbackAuth(cfg Config) gin.HandlerFunc {
	token := strings.TrimSpace(cfg.Token)
	maxSkew := cfg.MaxSkew
	if maxSkew <= 0 {
		maxSkew = defaultCallbackMaxSkew
	}
	now := cfg.Now
	if now == nil {
		now = time.Now
	}

	return func(c *gin.Context) {
		if token == "" {
			c.AbortWithStatusJSON(http.StatusServiceUnavailable, ginmw.SimpleErrorResponse{Error: "nexus callback auth not configured"})
			return
		}
		if provided := strings.TrimSpace(c.GetHeader("X-Internal-Service-Token")); subtle.ConstantTimeCompare([]byte(provided), []byte(token)) == 1 {
			c.Next()
			return
		}

		timestamp := strings.TrimSpace(c.GetHeader("X-Nexus-Callback-Timestamp"))
		signature := strings.TrimSpace(c.GetHeader("X-Nexus-Callback-Signature"))
		if timestamp == "" || signature == "" {
			c.AbortWithStatusJSON(http.StatusUnauthorized, ginmw.SimpleErrorResponse{Error: "unauthorized"})
			return
		}
		parsedTimestamp, err := time.Parse(time.RFC3339Nano, timestamp)
		if err != nil || outsideSkew(now().UTC(), parsedTimestamp.UTC(), maxSkew) {
			c.AbortWithStatusJSON(http.StatusUnauthorized, ginmw.SimpleErrorResponse{Error: "unauthorized"})
			return
		}

		body, err := io.ReadAll(c.Request.Body)
		if err != nil {
			c.AbortWithStatusJSON(http.StatusBadRequest, ginmw.SimpleErrorResponse{Error: "invalid callback body"})
			return
		}
		c.Request.Body = io.NopCloser(bytes.NewReader(body))

		expected := signNexusCallback(token, timestamp, body)
		if subtle.ConstantTimeCompare([]byte(signature), []byte(expected)) != 1 {
			c.AbortWithStatusJSON(http.StatusUnauthorized, ginmw.SimpleErrorResponse{Error: "unauthorized"})
			return
		}
		c.Next()
	}
}

func outsideSkew(now, timestamp time.Time, maxSkew time.Duration) bool {
	delta := now.Sub(timestamp)
	if delta < 0 {
		delta = -delta
	}
	return delta > maxSkew
}

func signNexusCallback(token, timestamp string, payload []byte) string {
	mac := hmac.New(sha256.New, []byte(token))
	_, _ = mac.Write([]byte(timestamp))
	_, _ = mac.Write([]byte("."))
	_, _ = mac.Write(payload)
	return "sha256=" + hex.EncodeToString(mac.Sum(nil))
}
