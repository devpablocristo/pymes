// Package reviewproxy proxies policy/approval requests from the frontend to Nexus Governance API.
// Usa el client genérico de core/governance/go/reviewclient internamente.
package reviewproxy

import (
	"github.com/devpablocristo/core/governance/go/reviewclient"
)

// Client wrapper sobre el client genérico de core.
type Client = reviewclient.Client

// NewClient crea un nuevo cliente HTTP hacia Nexus Governance.
var NewClient = reviewclient.NewClient
