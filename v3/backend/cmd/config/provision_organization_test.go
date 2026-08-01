package config

import "testing"

func TestLoadProvisionOrganizationFromKeepsSignedIdentityInLocalMode(t *testing.T) {
	t.Parallel()
	values := map[string]string{
		"PYMES_DATABASE_URL":                  " postgres://pymes ",
		"ACCOUNTING_PROVISIONING_URL":         " https://accounting-admin.example/ ",
		"PYMES_ENVIRONMENT":                   "test",
		"PYMES_ALLOW_INSECURE_LOCAL_SERVICES": "TRUE",
	}
	cfg, err := LoadProvisionOrganizationFrom(func(key string) string {
		return values[key]
	})
	if err != nil {
		t.Fatal(err)
	}
	if cfg.DatabaseURL != "postgres://pymes" ||
		cfg.AccountingProvisioningURL != "https://accounting-admin.example/" ||
		cfg.Environment != "test" ||
		!cfg.AllowInsecureLocalServices {
		t.Fatalf("config=%+v", cfg)
	}
}

func TestLoadProvisionOrganizationFromRejectsInvalidBoundaries(t *testing.T) {
	t.Parallel()
	base := map[string]string{
		"PYMES_DATABASE_URL":          "postgres://pymes",
		"ACCOUNTING_PROVISIONING_URL": "https://accounting-admin.example",
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
			name: "missing accounting admin",
			change: func(values map[string]string) {
				delete(values, "ACCOUNTING_PROVISIONING_URL")
			},
			code: "DEPENDENCY_URL_MISSING",
		},
		{
			name: "production local bypass",
			change: func(values map[string]string) {
				values["PYMES_ENVIRONMENT"] = "production"
				values["PYMES_ALLOW_INSECURE_LOCAL_SERVICES"] = "true"
			},
			code: "WORKLOAD_IDENTITY_INVALID",
		},
		{
			name: "unknown environment",
			change: func(values map[string]string) {
				values["PYMES_ENVIRONMENT"] = "staging"
			},
			code: "WORKLOAD_IDENTITY_INVALID",
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
			_, err := LoadProvisionOrganizationFrom(func(key string) string {
				return values[key]
			})
			if err == nil || ProvisionOrganizationErrorCode(err) != test.code {
				t.Fatalf(
					"err=%v code=%q",
					err,
					ProvisionOrganizationErrorCode(err),
				)
			}
		})
	}
}
