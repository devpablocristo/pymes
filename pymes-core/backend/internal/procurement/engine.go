package procurement

import (
	"github.com/devpablocristo/core/governance/go/approval"
	"github.com/devpablocristo/core/governance/go/decision"
	"github.com/devpablocristo/core/governance/go/risk"
)

// NewGovernanceEngine construye el motor decision/risk/approval de core/governance.
func NewGovernanceEngine() *decision.Engine {
	return decision.New(risk.DefaultConfig(), approval.DefaultConfig())
}
