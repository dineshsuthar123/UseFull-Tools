package compare

import (
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/local-first/what-changed/internal/snapshot"
)

type Result struct {
	Baseline SnapshotRef `json:"baseline"`
	Current  SnapshotRef `json:"current"`
	Findings []Finding   `json:"findings"`
	Skipped  []string    `json:"skippedDetectors,omitempty"`
}

type SnapshotRef struct {
	Label      string           `json:"label"`
	CapturedAt time.Time        `json:"capturedAt"`
	Root       string           `json:"root"`
	Trigger    snapshot.Trigger `json:"trigger"`
}

type Finding struct {
	Category  string `json:"category"`
	Change    string `json:"change"`
	Subject   string `json:"subject"`
	Before    string `json:"before,omitempty"`
	After     string `json:"after,omitempty"`
	Summary   string `json:"summary"`
	Score     int    `json:"score"`
	Why       string `json:"why"`
	Sensitive bool   `json:"sensitive,omitempty"`
}

func Snapshots(baseline, current *snapshot.Snapshot) Result {
	result := Result{
		Baseline: snapshotRef(baseline),
		Current:  snapshotRef(current),
		Findings: make([]Finding, 0),
	}
	compareFiles(&result, baseline, current)
	compareEnvironment(&result, baseline, current)
	compareRuntimes(&result, baseline, current)
	compareGit(&result, baseline, current)
	comparePorts(&result, baseline, current)
	compareContainers(&result, baseline, current)
	sort.Slice(result.Findings, func(i, j int) bool {
		left, right := result.Findings[i], result.Findings[j]
		if left.Score != right.Score {
			return left.Score > right.Score
		}
		if left.Category != right.Category {
			return left.Category < right.Category
		}
		if left.Subject != right.Subject {
			return left.Subject < right.Subject
		}
		return left.Change < right.Change
	})
	sort.Strings(result.Skipped)
	return result
}

func snapshotRef(value *snapshot.Snapshot) SnapshotRef {
	return SnapshotRef{Label: value.Label, CapturedAt: value.CapturedAt, Root: value.Root, Trigger: value.Trigger}
}

func compareFiles(result *Result, before, after *snapshot.Snapshot) {
	for path, oldState := range before.Files {
		newState, found := after.Files[path]
		if !found {
			if after.Complete["files"] {
				result.Findings = append(result.Findings, fileFinding("removed", path, oldState, snapshot.FileState{}))
			}
			continue
		}
		if oldState.SHA256 != newState.SHA256 {
			result.Findings = append(result.Findings, fileFinding("changed", path, oldState, newState))
		}
	}
	if before.Complete["files"] {
		for path, state := range after.Files {
			if _, found := before.Files[path]; !found {
				result.Findings = append(result.Findings, fileFinding("added", path, snapshot.FileState{}, state))
			}
		}
	}
	if !before.Complete["files"] || !after.Complete["files"] {
		result.Skipped = append(result.Skipped, "some file additions/removals (partial scan)")
	}
}

func fileFinding(change, path string, before, after snapshot.FileState) Finding {
	kind := before.Kind
	if kind == "" {
		kind = after.Kind
	}
	score, why := fileScore(kind)
	return Finding{
		Category: fileCategory(kind), Change: change, Subject: path,
		Before: fileDescription(before), After: fileDescription(after),
		Summary: fmt.Sprintf("%s %s", path, change), Score: score, Why: why,
	}
}

func fileCategory(kind string) string {
	switch kind {
	case "dependency":
		return "dependencies"
	case "migration":
		return "migrations"
	case "config":
		return "configuration"
	case "source":
		return "code"
	case "test":
		return "tests"
	default:
		return "files"
	}
}

func fileDescription(state snapshot.FileState) string {
	if state.SHA256 == "" {
		return ""
	}
	digest := state.SHA256
	if len(digest) > 12 {
		digest = digest[:12]
	}
	return fmt.Sprintf("sha256:%s, %d bytes", digest, state.Size)
}

func fileScore(kind string) (int, string) {
	switch kind {
	case "dependency":
		return 96, "dependency manifests and lockfiles can change the code that executes without an obvious source edit"
	case "migration":
		return 94, "migration changes can alter persistent application state"
	case "config":
		return 90, "configuration directly changes runtime behavior"
	case "source":
		return 86, "executable source changed since the successful checkpoint"
	case "test":
		return 72, "test code changed, which can change the observed result"
	default:
		return 50, "a tracked project file changed"
	}
}

func compareEnvironment(result *Result, before, after *snapshot.Snapshot) {
	if !before.Complete["environment"] || !after.Complete["environment"] {
		result.Skipped = append(result.Skipped, "environment")
		return
	}
	for name, oldState := range before.Environment {
		newState, found := after.Environment[name]
		if !found {
			result.Findings = append(result.Findings, environmentFinding("removed", name, oldState, snapshot.EnvState{}))
			continue
		}
		if oldState.SHA256 != newState.SHA256 {
			result.Findings = append(result.Findings, environmentFinding("changed", name, oldState, newState))
		}
	}
	for name, state := range after.Environment {
		if _, found := before.Environment[name]; !found {
			result.Findings = append(result.Findings, environmentFinding("added", name, snapshot.EnvState{}, state))
		}
	}
}

func environmentFinding(change, name string, before, after snapshot.EnvState) Finding {
	sensitive := before.Redacted || after.Redacted
	score, why := environmentScore(name, sensitive)
	return Finding{
		Category: "environment", Change: change, Subject: name,
		Before: envDescription(before), After: envDescription(after),
		Summary: fmt.Sprintf("environment variable %s %s", name, change),
		Score:   score, Why: why, Sensitive: sensitive,
	}
}

func envDescription(state snapshot.EnvState) string {
	if state.SHA256 == "" {
		return ""
	}
	if state.Redacted {
		return "<redacted>"
	}
	return truncate(state.Value, 160)
}

func environmentScore(name string, sensitive bool) (int, string) {
	upper := strings.ToUpper(name)
	if sensitive {
		return 82, "a credential-like environment value changed; its plaintext was not stored"
	}
	important := []string{"CONFIG", "DATABASE", "DB_", "ENV", "HOST", "JAVA", "LANG", "NODE", "POOL", "PORT", "PROFILE", "PYTHON", "REDIS", "REGION", "TIMEZONE", "TZ", "URL"}
	for _, fragment := range important {
		if strings.Contains(upper, fragment) {
			return 84, "this environment variable looks runtime- or project-relevant"
		}
	}
	if upper == "PATH" || strings.HasSuffix(upper, "PATH") {
		return 78, "executable or library lookup paths changed"
	}
	return 62, "an inherited environment value changed"
}

func compareRuntimes(result *Result, before, after *snapshot.Snapshot) {
	if !before.Complete["runtimes"] || !after.Complete["runtimes"] {
		result.Skipped = append(result.Skipped, "runtimes")
		return
	}
	compareSimpleMaps(before.Runtimes, after.Runtimes, func(change, name string, oldState, newState snapshot.RuntimeState) {
		result.Findings = append(result.Findings, Finding{
			Category: "runtimes", Change: change, Subject: name,
			Before: oldState.Version, After: newState.Version,
			Summary: fmt.Sprintf("%s runtime %s", name, change), Score: 88,
			Why: "runtime version or availability can change build and execution behavior",
		})
	})
}

func compareGit(result *Result, before, after *snapshot.Snapshot) {
	if !before.Complete["git"] || !after.Complete["git"] || before.Git == nil || after.Git == nil {
		if before.Complete["git"] != after.Complete["git"] {
			result.Skipped = append(result.Skipped, "git")
		}
		return
	}
	if before.Git.Branch != after.Git.Branch {
		result.Findings = append(result.Findings, Finding{
			Category: "git", Change: "changed", Subject: "branch",
			Before: before.Git.Branch, After: after.Git.Branch,
			Summary: "Git branch changed", Score: 92,
			Why: "a branch switch can replace many inputs at once",
		})
	}
	if before.Git.Commit != after.Git.Commit {
		result.Findings = append(result.Findings, Finding{
			Category: "git", Change: "changed", Subject: "commit",
			Before: shortCommit(before.Git.Commit), After: shortCommit(after.Git.Commit),
			Summary: "Git commit changed", Score: 68,
			Why: "the checked-out repository revision changed",
		})
	}
}

func comparePorts(result *Result, before, after *snapshot.Snapshot) {
	if !before.Complete["ports"] || !after.Complete["ports"] {
		result.Skipped = append(result.Skipped, "listening ports")
		return
	}
	for key, oldState := range before.Ports {
		newState, found := after.Ports[key]
		if !found {
			result.Findings = append(result.Findings, portFinding("freed", key, oldState, snapshot.PortState{}))
			continue
		}
		if oldState.Owner != newState.Owner && oldState.Owner != "" && newState.Owner != "" {
			result.Findings = append(result.Findings, portFinding("owner changed", key, oldState, newState))
		}
	}
	for key, state := range after.Ports {
		if _, found := before.Ports[key]; !found {
			result.Findings = append(result.Findings, portFinding("occupied", key, snapshot.PortState{}, state))
		}
	}
}

var commonDevelopmentPorts = map[int]struct{}{
	3000: {}, 3001: {}, 3306: {}, 4200: {}, 5000: {}, 5173: {}, 5432: {}, 6379: {},
	8000: {}, 8080: {}, 8081: {}, 8090: {}, 9000: {}, 9200: {}, 27017: {},
}

func portFinding(change, key string, before, after snapshot.PortState) Finding {
	state := before
	if state.Port == 0 {
		state = after
	}
	score := 55
	why := "a listening TCP port changed"
	if _, common := commonDevelopmentPorts[state.Port]; common {
		score = 82
		why = "a commonly used development or database port changed state"
	}
	return Finding{
		Category: "ports", Change: change, Subject: key,
		Before: portDescription(before), After: portDescription(after),
		Summary: fmt.Sprintf("TCP port %d %s", state.Port, change), Score: score, Why: why,
	}
}

func portDescription(state snapshot.PortState) string {
	if state.Port == 0 {
		return ""
	}
	value := state.Address + ":" + strconv.Itoa(state.Port)
	if state.Owner != "" {
		value += " (" + state.Owner + ")"
	}
	return value
}

func compareContainers(result *Result, before, after *snapshot.Snapshot) {
	if !before.Complete["containers"] || !after.Complete["containers"] {
		return
	}
	compareSimpleMaps(before.Containers, after.Containers, func(change, name string, oldState, newState snapshot.ContainerState) {
		if change == "unchanged" {
			return
		}
		result.Findings = append(result.Findings, Finding{
			Category: "containers", Change: change, Subject: name,
			Before: containerDescription(oldState), After: containerDescription(newState),
			Summary: fmt.Sprintf("container %s %s", name, change), Score: 78,
			Why: "container image or running state changed",
		})
	})
}

func containerDescription(state snapshot.ContainerState) string {
	if state.Image == "" && state.State == "" {
		return ""
	}
	return strings.TrimSpace(state.Image + " (" + state.State + ")")
}

func compareSimpleMaps[T comparable](before, after map[string]T, add func(change, name string, before, after T)) {
	var zero T
	for name, oldState := range before {
		newState, found := after[name]
		if !found {
			add("removed", name, oldState, zero)
			continue
		}
		if oldState != newState {
			add("changed", name, oldState, newState)
		}
	}
	for name, state := range after {
		if _, found := before[name]; !found {
			add("added", name, zero, state)
		}
	}
}

func shortCommit(value string) string {
	if len(value) > 12 {
		return value[:12]
	}
	return value
}

func truncate(value string, limit int) string {
	if len(value) <= limit {
		return value
	}
	return value[:limit-1] + "…"
}
