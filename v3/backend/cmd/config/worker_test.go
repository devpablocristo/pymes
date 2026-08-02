package config

import (
	"testing"
	"time"
)

func TestLoadWorkerFromPreservesSecureRuntimeDefaults(t *testing.T) {
	t.Parallel()
	values := map[string]string{
		"PYMES_DATABASE_URL":                   "postgres://db",
		"FISCAL_ADAPTER_URL":                   "http://fiscal",
		"ACCOUNTING_URL":                       "http://accounting",
		"PYMES_SCHEDULING_ACTION_TOKEN_SECRET": "01234567890123456789012345678901",
	}
	cfg, err := LoadWorkerFrom(func(key string) string {
		return values[key]
	})
	if err != nil {
		t.Fatal(err)
	}
	if cfg.HTTPAddr != ":8080" ||
		cfg.Environment != "development" ||
		cfg.ReleaseSHA != localWorkerReleaseSHA ||
		cfg.Revision != localWorkerRevision ||
		cfg.AllowInsecureLocalServices ||
		cfg.RunOnce ||
		cfg.DispatchInterval != time.Second ||
		cfg.MetricsInterval != time.Minute ||
		cfg.LeaseDuration != 30*time.Second ||
		cfg.ShutdownTimeout != 5*time.Second {
		t.Fatalf("worker config=%+v", cfg)
	}
}

func TestLoadWorkerFromRequiresExactReleaseMetadataInProduction(t *testing.T) {
	t.Parallel()
	base := map[string]string{
		"PYMES_DATABASE_URL":                   "postgres://db",
		"FISCAL_ADAPTER_URL":                   "https://fiscal",
		"ACCOUNTING_URL":                       "https://accounting",
		"PYMES_SCHEDULING_ACTION_TOKEN_SECRET": "01234567890123456789012345678901",
		"PYMES_ENVIRONMENT":                    "production",
		"PYMES_RELEASE_SHA":                    "0123456789abcdef0123456789abcdef01234567",
		"K_REVISION":                           "pymes-v3-stg-worker-00042-abc",
	}
	cfg, err := LoadWorkerFrom(func(key string) string { return base[key] })
	if err != nil {
		t.Fatal(err)
	}
	if cfg.ReleaseSHA != base["PYMES_RELEASE_SHA"] ||
		cfg.Revision != base["K_REVISION"] {
		t.Fatalf("release metadata = %q %q", cfg.ReleaseSHA, cfg.Revision)
	}

	tests := []struct {
		name   string
		change func(map[string]string)
	}{
		{
			name: "missing release SHA",
			change: func(values map[string]string) {
				delete(values, "PYMES_RELEASE_SHA")
			},
		},
		{
			name: "uppercase release SHA",
			change: func(values map[string]string) {
				values["PYMES_RELEASE_SHA"] = "0123456789ABCDEF0123456789abcdef01234567"
			},
		},
		{
			name: "missing revision",
			change: func(values map[string]string) {
				delete(values, "K_REVISION")
			},
		},
		{
			name: "invalid revision",
			change: func(values map[string]string) {
				values["K_REVISION"] = "pymes/worker"
			},
		},
	}
	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			values := make(map[string]string, len(base))
			for key, value := range base {
				values[key] = value
			}
			test.change(values)
			_, err := LoadWorkerFrom(func(key string) string {
				return values[key]
			})
			if err == nil ||
				WorkerErrorCode(err) != "WORKER_RELEASE_METADATA_INVALID" {
				t.Fatalf("err=%v code=%q", err, WorkerErrorCode(err))
			}
		})
	}
}

func TestLoadWorkerFromValidatesIdentityDependenciesAndIntervals(t *testing.T) {
	t.Parallel()
	base := map[string]string{
		"PYMES_DATABASE_URL":                   "postgres://db",
		"FISCAL_ADAPTER_URL":                   "http://fiscal",
		"ACCOUNTING_URL":                       "http://accounting",
		"PYMES_SCHEDULING_ACTION_TOKEN_SECRET": "01234567890123456789012345678901",
	}
	tests := []struct {
		name   string
		change func(map[string]string)
		code   string
	}{
		{
			name: "missing database",
			change: func(values map[string]string) {
				delete(values, "PYMES_DATABASE_URL")
			},
			code: "DATABASE_URL_MISSING",
		},
		{
			name: "missing accounting dependency",
			change: func(values map[string]string) {
				delete(values, "ACCOUNTING_URL")
			},
			code: "DEPENDENCY_URL_MISSING",
		},
		{
			name: "missing fiscal dependency",
			change: func(values map[string]string) {
				delete(values, "FISCAL_ADAPTER_URL")
			},
			code: "DEPENDENCY_URL_MISSING",
		},
		{
			name: "missing scheduling action token secret",
			change: func(values map[string]string) {
				delete(values, "PYMES_SCHEDULING_ACTION_TOKEN_SECRET")
			},
			code: "ACTION_TOKEN_SECRET_INVALID",
		},
		{
			name: "production insecure bypass",
			change: func(values map[string]string) {
				values["PYMES_ENVIRONMENT"] = "production"
				values["PYMES_RELEASE_SHA"] = "0123456789abcdef0123456789abcdef01234567"
				values["K_REVISION"] = "pymes-v3-stg-worker-00042-abc"
				values["PYMES_ALLOW_INSECURE_LOCAL_SERVICES"] = "true"
			},
			code: "WORKLOAD_IDENTITY_INVALID",
		},
		{
			name: "invalid metrics interval",
			change: func(values map[string]string) {
				values["PYMES_WORKER_METRICS_INTERVAL"] = "4s"
			},
			code: "METRICS_INTERVAL_INVALID",
		},
		{
			name: "invalid one-shot flag",
			change: func(values map[string]string) {
				values["PYMES_WORKER_RUN_ONCE"] = "eventually"
			},
			code: "WORKER_RUN_ONCE_INVALID",
		},
	}
	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			values := make(map[string]string, len(base))
			for key, value := range base {
				values[key] = value
			}
			test.change(values)
			_, err := LoadWorkerFrom(func(key string) string {
				return values[key]
			})
			if err == nil || WorkerErrorCode(err) != test.code {
				t.Fatalf("err=%v code=%q", err, WorkerErrorCode(err))
			}
		})
	}
}

func TestLoadWorkerFromPreservesFastLocalLoop(t *testing.T) {
	t.Parallel()
	values := map[string]string{
		"PYMES_DATABASE_URL":                   "postgres://db",
		"FISCAL_ADAPTER_URL":                   "http://fiscal",
		"ACCOUNTING_URL":                       "http://accounting",
		"PYMES_SCHEDULING_ACTION_TOKEN_SECRET": "01234567890123456789012345678901",
		"PYMES_ENVIRONMENT":                    "test",
		"PYMES_ALLOW_INSECURE_LOCAL_SERVICES":  "TRUE",
		"PYMES_WORKER_INTERVAL_MS":             "250",
		"PYMES_WORKER_METRICS_INTERVAL":        "15s",
		"PYMES_WORKER_RUN_ONCE":                "TRUE",
	}
	cfg, err := LoadWorkerFrom(func(key string) string {
		return values[key]
	})
	if err != nil {
		t.Fatal(err)
	}
	if !cfg.AllowInsecureLocalServices ||
		!cfg.RunOnce ||
		cfg.ReleaseSHA != localWorkerReleaseSHA ||
		cfg.Revision != localWorkerRevision ||
		cfg.DispatchInterval != 250*time.Millisecond ||
		cfg.MetricsInterval != 15*time.Second {
		t.Fatalf("worker config=%+v", cfg)
	}
}

func TestLoadWorkerFromValidatesPerGoOnlyWhenEnabled(t *testing.T) {
	values := map[string]string{
		"PYMES_DATABASE_URL":                   "postgres://db",
		"FISCAL_ADAPTER_URL":                   "http://fiscal",
		"ACCOUNTING_URL":                       "http://accounting",
		"PYMES_SCHEDULING_ACTION_TOKEN_SECRET": "01234567890123456789012345678901",
		"PYMES_PERGO_ENABLED":                  "true",
		"PERGO_URL":                            "http://pergo/",
		"PERGO_API_KEY":                        "secret",
		"PERGO_WORKSPACE_ID":                   "workspace-1",
		"PERGO_CHANNEL":                        "whatsapp_mock",
		"PERGO_ALLOW_GLOBAL_ROUTE_FALLBACK":    "true",
		"PERGO_TIMEOUT":                        "750ms",
	}
	cfg, err := LoadWorkerFrom(func(key string) string { return values[key] })
	if err != nil {
		t.Fatal(err)
	}
	if !cfg.PerGo.Enabled || cfg.PerGo.BaseURL != "http://pergo" ||
		cfg.PerGo.Timeout != 750*time.Millisecond ||
		!cfg.PerGo.AllowGlobalRouteFallback {
		t.Fatalf("PerGo config = %+v", cfg.PerGo)
	}
	delete(values, "PERGO_API_KEY")
	if _, err := LoadWorkerFrom(func(key string) string { return values[key] }); err == nil ||
		WorkerErrorCode(err) != "PERGO_CONFIG_INVALID" {
		t.Fatalf("missing key error = %v", err)
	}
}
