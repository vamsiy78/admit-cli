package resolver

import (
	"testing"

	"admit/internal/schema"
)

func TestResolve_BasicFunctionality(t *testing.T) {
	s := schema.Schema{
		Config: map[string]schema.ConfigKey{
			"db.url": {
				Path:     "db.url",
				Type:     schema.TypeString,
				Required: true,
			},
			"payments.mode": {
				Path:     "payments.mode",
				Type:     schema.TypeEnum,
				Required: true,
				Values:   []string{"test", "live"},
			},
		},
	}

	environ := []string{
		"DB_URL=postgres://localhost/mydb",
		"PAYMENTS_MODE=test",
		"OTHER_VAR=ignored",
	}

	results := Resolve(s, environ)

	if len(results) != 2 {
		t.Fatalf("expected 2 results, got %d", len(results))
	}

	// Build a map for easier assertions
	resultMap := make(map[string]ResolvedValue)
	for _, r := range results {
		resultMap[r.Key] = r
	}

	// Check db.url resolution
	dbURL, ok := resultMap["db.url"]
	if !ok {
		t.Fatal("missing db.url in results")
	}
	if dbURL.EnvVar != "DB_URL" {
		t.Errorf("expected EnvVar DB_URL, got %s", dbURL.EnvVar)
	}
	if dbURL.Value != "postgres://localhost/mydb" {
		t.Errorf("expected value postgres://localhost/mydb, got %s", dbURL.Value)
	}
	if !dbURL.Present {
		t.Error("expected Present to be true for db.url")
	}

	// Check payments.mode resolution
	paymentsMode, ok := resultMap["payments.mode"]
	if !ok {
		t.Fatal("missing payments.mode in results")
	}
	if paymentsMode.EnvVar != "PAYMENTS_MODE" {
		t.Errorf("expected EnvVar PAYMENTS_MODE, got %s", paymentsMode.EnvVar)
	}
	if paymentsMode.Value != "test" {
		t.Errorf("expected value test, got %s", paymentsMode.Value)
	}
	if !paymentsMode.Present {
		t.Error("expected Present to be true for payments.mode")
	}
}

func TestResolve_MissingEnvVar(t *testing.T) {
	s := schema.Schema{
		Config: map[string]schema.ConfigKey{
			"db.url": {
				Path:     "db.url",
				Type:     schema.TypeString,
				Required: true,
			},
		},
	}

	// Empty environ - no env vars set
	environ := []string{}

	results := Resolve(s, environ)

	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}

	r := results[0]
	if r.Present {
		t.Error("expected Present to be false for missing env var")
	}
	if r.Value != "" {
		t.Errorf("expected empty value, got %s", r.Value)
	}
}

func TestResolve_EmptyValue(t *testing.T) {
	s := schema.Schema{
		Config: map[string]schema.ConfigKey{
			"db.url": {
				Path:     "db.url",
				Type:     schema.TypeString,
				Required: true,
			},
		},
	}

	// Env var set but empty
	environ := []string{"DB_URL="}

	results := Resolve(s, environ)

	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}

	r := results[0]
	if !r.Present {
		t.Error("expected Present to be true for set but empty env var")
	}
	if r.Value != "" {
		t.Errorf("expected empty value, got %s", r.Value)
	}
}

func TestResolve_ValueWithEquals(t *testing.T) {
	s := schema.Schema{
		Config: map[string]schema.ConfigKey{
			"db.url": {
				Path:     "db.url",
				Type:     schema.TypeString,
				Required: true,
			},
		},
	}

	// Value contains "=" character
	environ := []string{"DB_URL=postgres://user:pass=word@localhost/db"}

	results := Resolve(s, environ)

	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}

	r := results[0]
	if r.Value != "postgres://user:pass=word@localhost/db" {
		t.Errorf("expected value with = preserved, got %s", r.Value)
	}
}
