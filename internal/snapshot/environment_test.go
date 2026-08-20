package snapshot

import "testing"

func TestSensitiveEnvironmentNames(t *testing.T) {
	tests := map[string]bool{
		"OPENAI_API_KEY":          true,
		"AWS_ACCESS_KEY_ID":       true,
		"DATABASE_URL":            true,
		"REDIS_URL":               true,
		"PAYMENT_REGION":          false,
		"REDIS_POOL_SIZE":         false,
		"JAVA_HOME":               false,
		"MONKEY_PATCH_ENABLED":    false,
		"PUBLIC_API_ENDPOINT_URL": false,
	}
	for name, want := range tests {
		if got := isSensitiveEnvName(name); got != want {
			t.Errorf("isSensitiveEnvName(%q)=%v, want %v", name, got, want)
		}
	}
}
