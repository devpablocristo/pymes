package helpers

import "testing"

func TestValidateTargetURLCanonicalTarget(t *testing.T) {
	t.Parallel()
	target, err := PymesTarget("stg")
	if err != nil {
		t.Fatal(err)
	}
	valid := "postgres://pymes_v3_migrate_stg:0123456789abcdef@/pymes_v3_stg" +
		"?host=/cloudsql/pymes-dev-352318:us-central1:pymes-dev-db" +
		"&sslmode=disable&options=-c%20role%3Dpymes_v3_owner_stg"
	if err := ValidateTargetURL(valid, target); err != nil {
		t.Fatalf("canonical URL rejected: %v", err)
	}

	tests := map[string]string{
		"foreign host": "postgres://pymes_v3_migrate_stg:secret@other.example/pymes_v3_stg" +
			"?sslmode=require&options=-c%20role%3Dpymes_v3_owner_stg",
		"foreign socket": "postgres://pymes_v3_migrate_stg:secret@/pymes_v3_stg" +
			"?host=/cloudsql/other:us-central1:db&sslmode=disable" +
			"&options=-c%20role%3Dpymes_v3_owner_stg",
		"wrong database": "postgres://pymes_v3_migrate_stg:secret@/other" +
			"?host=/cloudsql/pymes-dev-352318:us-central1:pymes-dev-db" +
			"&sslmode=disable&options=-c%20role%3Dpymes_v3_owner_stg",
		"wrong session role": "postgres://other:secret@/pymes_v3_stg" +
			"?host=/cloudsql/pymes-dev-352318:us-central1:pymes-dev-db" +
			"&sslmode=disable&options=-c%20role%3Dpymes_v3_owner_stg",
		"wrong effective role": "postgres://pymes_v3_migrate_stg:secret@/pymes_v3_stg" +
			"?host=/cloudsql/pymes-dev-352318:us-central1:pymes-dev-db" +
			"&sslmode=disable&options=-c%20role%3Dother",
		"extra parameter": valid + "&application_name=other",
	}
	for name, databaseURL := range tests {
		databaseURL := databaseURL
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			if err := ValidateTargetURL(databaseURL, target); err == nil {
				t.Fatal("unsafe database URL accepted")
			}
		})
	}
}

func TestValidateTargetIdentityRejectsMismatch(t *testing.T) {
	t.Parallel()
	target, err := PymesTarget("prd")
	if err != nil {
		t.Fatal(err)
	}
	if err := ValidateTargetIdentity(
		target,
		target.Database,
		target.SessionRole,
		target.EffectiveRole,
	); err != nil {
		t.Fatalf("canonical identity rejected: %v", err)
	}
	if err := ValidateTargetIdentity(
		target,
		"other",
		target.SessionRole,
		target.EffectiveRole,
	); err == nil {
		t.Fatal("foreign database identity accepted")
	}
}

func TestPymesTargetRejectsUnknownEnvironment(t *testing.T) {
	t.Parallel()
	if _, err := PymesTarget("production"); err == nil {
		t.Fatal("unknown migration environment accepted")
	}
}
