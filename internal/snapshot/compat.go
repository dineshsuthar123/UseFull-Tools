package snapshot

import "strings"

func UpgradeV1(value *Snapshot) {
	if value == nil || value.SchemaVersion != 1 {
		return
	}
	for path, state := range value.Files {
		state.Tracked = state.SHA256 != ""
		if !state.Tracked && state.Reason == "" {
			state.Reason = "legacy-untracked"
		}
		value.Files[path] = state
	}
	for name, state := range value.Environment {
		upper := strings.ToUpper(name)
		if _, safe := safePlaintextEnvironment[upper]; safe {
			state.Sensitivity = "safe"
		} else {
			state.Value = ""
			if isSensitiveEnvName(upper) || state.Redacted {
				state.Sensitivity = "secret-name"
			} else {
				state.Sensitivity = "unknown"
			}
		}
		state.Redacted = false
		value.Environment[name] = state
	}
	if value.Complete == nil {
		value.Complete = map[string]bool{}
	}
	value.Complete["projectContext"] = false
	value.SchemaVersion = SchemaVersion
}
