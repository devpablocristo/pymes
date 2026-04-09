package whatsapp

import cm "github.com/devpablocristo/pymes/pymes-core/backend/internal/customer_messaging"

type Connection = cm.Connection
type TokenCrypto = cm.TokenCrypto
type AIClientPort = cm.AIClientPort
type MetaClientPort = cm.MetaClientPort
type InboundMessage = cm.InboundMessage
type InboundResult = cm.InboundResult
type AIMessageResponse = cm.AIMessageResponse
type InteractiveButtonPayload = cm.InteractiveButtonPayload

func parseInboundMessages(payload []byte) ([]InboundMessage, error) {
	return cm.ParseInboundMessages(payload)
}
