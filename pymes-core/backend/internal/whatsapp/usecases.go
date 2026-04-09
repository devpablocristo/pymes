package whatsapp

import cm "github.com/devpablocristo/pymes/pymes-core/backend/internal/customer_messaging"

type RepositoryPort = cm.RepositoryPort
type TimelinePort = cm.TimelinePort
type QuoteSnapshot = cm.QuoteSnapshot
type SaleSnapshot = cm.SaleSnapshot
type Templates = cm.Templates
type Result = cm.Result

type Usecases struct {
	*cm.Usecases
}

func NewUsecases(repo RepositoryPort, timeline TimelinePort, frontendURL string, ai AIClientPort, meta MetaClientPort, tokenCrypto TokenCrypto, webhookVerifyToken, webhookAppSecret string) *Usecases {
	return &Usecases{
		Usecases: cm.NewUsecases(repo, timeline, frontendURL, ai, meta, tokenCrypto, webhookVerifyToken, webhookAppSecret),
	}
}
