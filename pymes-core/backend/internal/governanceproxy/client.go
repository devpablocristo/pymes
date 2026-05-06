// Package governanceproxy proxies policy/approval requests from the frontend to Nexus Governance API.
// Usa el client genérico de core/governance/go/governanceclient internamente.
package governanceproxy

import (
	"github.com/devpablocristo/core/governance/go/governanceclient"
)

// Client wrapper sobre el client genérico de core.
type Client = governanceclient.Client

// NewClient crea un nuevo cliente HTTP hacia Nexus Governance.
var NewClient = governanceclient.NewClient
