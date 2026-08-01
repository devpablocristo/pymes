package config

import (
	"testing"
	"time"
)

func TestLoadWorkerFromPreservesSecureRuntimeDefaults(t *testing.T) {
	t.Parallel()
	values := map[string]string{
		"PYMES_DATABASE_URL": "postgres://db",
		"FISCAL_ADAPTER_URL": "http://fiscal",
		"ACCOUNTING_URL":     "http://accounting",
	}
	cfg, err := LoadWorkerFrom(func(key string) string {
		return values[key]
	})
	if err != nil {
		t.Fatal(err)
	}
	if cfg.HTTPAddr != ":8080" ||
		cfg.Environment != "development" ||
		cfg.AllowInsecureLocalServices ||
		cfg.RunOnce ||
		cfg.DispatchInterval != time.Second ||
		cfg.MetricsInterval != time.Minute ||
		cfg.LeaseDuration != 30*time.Second ||
		cfg.ShutdownTimeout != 5*time.Second {
		t.Fatalf("worker config=%+v", cfg)
	}
}

func TestLoadWorkerFromValidatesIdentityDependenciesAndIntervals(t *testing.T) {
	t.Parallel()
	base := map[string]string{
		"PYMES_DATABASE_URL": "postgres://db",
		"FISCAL_ADAPTER_URL": "http://fiscal",
		"ACCOUNTING_URL":     "http://accounting",
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
			name: "missing dependency",
			change: func(values map[string]string) {
				delete(values, "ACCOUNTING_URL")
			},
			code: "DEPENDENCY_URL_MISSING",
		},
		{
			name: "production insecure bypass",
			change: func(values map[string]string) {
				values["PYMES_ENVIRONMENT"] = "production"
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
		"PYMES_DATABASE_URL":                  "postgres://db",
		"FISCAL_ADAPTER_URL":                  "http://fiscal",
		"ACCOUNTING_URL":                      "http://accounting",
		"PYMES_ENVIRONMENT":                   "test",
		"PYMES_ALLOW_INSECURE_LOCAL_SERVICES": "TRUE",
		"PYMES_WORKER_INTERVAL_MS":            "250",
		"PYMES_WORKER_METRICS_INTERVAL":       "15s",
		"PYMES_WORKER_RUN_ONCE":               "TRUE",
	}
	cfg, err := LoadWorkerFrom(func(key string) string {
		return values[key]
	})
	if err != nil {
		t.Fatal(err)
	}
	if !cfg.AllowInsecureLocalServices ||
		!cfg.RunOnce ||
		cfg.DispatchInterval != 250*time.Millisecond ||
		cfg.MetricsInterval != 15*time.Second {
		t.Fatalf("worker config=%+v", cfg)
	}
}
