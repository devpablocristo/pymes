package config

import (
	"github.com/devpablocristo/pymes/pymes-core/shared/backend/verticalconfig"
)

type Config = verticalconfig.Config

func LoadFromEnv() Config {
	return verticalconfig.Load(verticalconfig.Options{DefaultPort: "8083"})
}
