package config

import (
	"github.com/devpablocristo/pymes/pymes-core/shared/backend/verticalconfig"
)

// Config centraliza la configuracion externa para mantener el mismo codigo entre ambientes.
type Config = verticalconfig.Config

// LoadFromEnv carga valores con defaults seguros para desarrollo local.
func LoadFromEnv() Config {
	return verticalconfig.Load(verticalconfig.Options{DefaultPort: "8081"})
}
