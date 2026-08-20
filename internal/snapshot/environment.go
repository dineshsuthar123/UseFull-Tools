package snapshot

import (
	"crypto/sha256"
	"encoding/hex"
	"os"
	"sort"
	"strings"
)

var sensitiveEnvFragments = []string{
	"ACCESS_KEY", "API_KEY", "APIKEY", "AUTH", "CONNECTION_STRING", "COOKIE", "CREDENTIAL",
	"DATABASE_URL", "DB_URL", "DSN", "JWT", "MONGO_URI", "PASS", "PRIVATE", "REDIS_URL",
	"SECRET", "SESSION", "SIGNING", "TOKEN", "BEARER", "CERTIFICATE",
}

var safePlaintextEnvironment = map[string]struct{}{
	"ASPNETCORE_ENVIRONMENT": {}, "CI": {}, "DJANGO_SETTINGS_MODULE": {}, "DOTNET_ENVIRONMENT": {},
	"FLASK_ENV": {}, "GOARCH": {}, "GOOS": {}, "LANG": {}, "LC_ALL": {}, "NODE_ENV": {},
	"RACK_ENV": {}, "RAILS_ENV": {}, "SPRING_PROFILES_ACTIVE": {}, "TZ": {},
}

var ignoredEnvironment = map[string]struct{}{
	"_": {}, "OLDPWD": {}, "PROMPT": {}, "PROMPT_COMMAND": {}, "PS1": {}, "PS2": {},
	"RANDOM": {}, "SECONDS": {}, "SHLVL": {}, "TERM_SESSION_ID": {},
}

func captureEnvironment() map[string]EnvState {
	result := make(map[string]EnvState)
	entries := os.Environ()
	sort.Strings(entries)
	for _, entry := range entries {
		name, value, found := strings.Cut(entry, "=")
		if !found || name == "" {
			continue
		}
		upper := strings.ToUpper(name)
		if _, ignored := ignoredEnvironment[upper]; ignored {
			continue
		}
		digest := sha256.Sum256([]byte(value))
		state := EnvState{SHA256: hex.EncodeToString(digest[:]), Sensitivity: "unknown"}
		if _, safe := safePlaintextEnvironment[upper]; safe {
			state.Sensitivity = "safe"
			state.Value = value
		} else if isSensitiveEnvName(upper) {
			state.Sensitivity = "secret-name"
		}
		result[name] = state
	}
	return result
}

func isSensitiveEnvName(upper string) bool {
	compact := strings.NewReplacer("_", "", "-", "", ".", "").Replace(upper)
	compactFragments := []string{"ACCESSKEY", "APIKEY", "CLIENTSECRET", "CONNECTIONSTRING", "PASSWORD", "PASSPHRASE", "PRIVATEKEY"}
	for _, fragment := range compactFragments {
		if strings.Contains(compact, fragment) {
			return true
		}
	}
	if strings.HasSuffix(upper, "_PAT") || strings.HasPrefix(upper, "PAT_") {
		return true
	}
	for _, fragment := range sensitiveEnvFragments {
		if strings.Contains(upper, fragment) {
			return true
		}
	}
	return false
}
