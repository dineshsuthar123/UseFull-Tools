package snapshot

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestSensitiveEnvironmentNames(t *testing.T) {
	tests := map[string]bool{
		"OPENAI_API_KEY":                 true,
		"AWS_ACCESS_KEY_ID":              true,
		"DATABASE_URL":                   true,
		"REDIS_URL":                      true,
		"PAYMENT_REGION":                 false,
		"REDIS_POOL_SIZE":                false,
		"JAVA_HOME":                      false,
		"MONKEY_PATCH_ENABLED":           false,
		"PUBLIC_API_ENDPOINT_URL":        false,
		"PGPASSWORD":                     true,
		"GITHUB_PAT":                     true,
		"AZURE_STORAGE_CONNECTIONSTRING": true,
		"CLIENTSECRET":                   true,
	}
	for name, want := range tests {
		if got := isSensitiveEnvName(name); got != want {
			t.Errorf("isSensitiveEnvName(%q)=%v, want %v", name, got, want)
		}
	}
}

func TestEnvironmentStoresPlaintextOnlyForExplicitAllowlist(t *testing.T) {
	t.Setenv("WHAT_CHANGED_UNKNOWN_VALUE", "private-project-value")
	t.Setenv("WHAT_CHANGED_TOKEN", "synthetic-secret-value")
	t.Setenv("NODE_ENV", "test")
	states := captureEnvironment()
	if got := states["WHAT_CHANGED_UNKNOWN_VALUE"]; got.Value != "" || got.Sensitivity != "unknown" {
		t.Fatalf("unknown variable stored plaintext or wrong metadata: %#v", got)
	}
	if got := states["WHAT_CHANGED_TOKEN"]; got.Value != "" || got.Sensitivity != "secret-name" {
		t.Fatalf("secret variable stored plaintext or wrong metadata: %#v", got)
	}
	if got := states["NODE_ENV"]; got.Value != "test" || got.Sensitivity != "safe" {
		t.Fatalf("safe variable was not preserved: %#v", got)
	}
	encoded, err := json.Marshal(states)
	if err != nil {
		t.Fatal(err)
	}
	for _, forbidden := range []string{"private-project-value", "synthetic-secret-value"} {
		if strings.Contains(string(encoded), forbidden) {
			t.Fatalf("checkpoint environment JSON contains %q: %s", forbidden, encoded)
		}
	}
}
