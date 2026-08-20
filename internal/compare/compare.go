package compare

import (
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/dineshsuthar123/UseFull-Tools/internal/snapshot"
)

type Result struct {
	Baseline    SnapshotRef           `json:"baseline"`
	Current     SnapshotRef           `json:"current"`
	Findings    []Finding             `json:"findings"`
	Skipped     []string              `json:"skippedDetectors,omitempty"`
	Diagnostics []snapshot.Diagnostic `json:"detectorDiagnostics,omitempty"`
}

type SnapshotRef struct {
	Label      string           `json:"label"`
	CommandID  string           `json:"commandId,omitempty"`
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

type projectRelevance struct {
	languages    map[string]struct{}
	buildSystems map[string]struct{}
	environment  map[string]snapshot.EnvReference
	ports        map[int]snapshot.PortRef
	services     map[string]snapshot.ServiceRef
}

func Snapshots(baseline, current *snapshot.Snapshot) Result {
	result := Result{
		Baseline: snapshotRef(baseline), Current: snapshotRef(current), Findings: make([]Finding, 0),
		Diagnostics: append([]snapshot.Diagnostic(nil), current.Diagnostics...),
	}
	context := buildRelevance(baseline, current)
	compareFiles(&result, baseline, current)
	compareEnvironment(&result, baseline, current, context)
	compareRuntimes(&result, baseline, current, context)
	compareGit(&result, baseline, current)
	comparePorts(&result, baseline, current, context)
	compareContainers(&result, baseline, current)
	if !baseline.Complete["projectContext"] && !current.Complete["projectContext"] {
		result.Skipped = append(result.Skipped, "project context")
	}
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
	result.Skipped = uniqueSorted(result.Skipped)
	sort.Slice(result.Diagnostics, func(i, j int) bool {
		if result.Diagnostics[i].Detector != result.Diagnostics[j].Detector {
			return result.Diagnostics[i].Detector < result.Diagnostics[j].Detector
		}
		if result.Diagnostics[i].Severity != result.Diagnostics[j].Severity {
			return result.Diagnostics[i].Severity < result.Diagnostics[j].Severity
		}
		return result.Diagnostics[i].Message < result.Diagnostics[j].Message
	})
	return result
}

func snapshotRef(value *snapshot.Snapshot) SnapshotRef {
	return SnapshotRef{Label: value.Label, CommandID: value.CommandID, CapturedAt: value.CapturedAt, Root: value.Root, Trigger: value.Trigger}
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
		oldTracked, newTracked := tracked(oldState), tracked(newState)
		switch {
		case oldTracked && newTracked && oldState.SHA256 != newState.SHA256:
			result.Findings = append(result.Findings, fileFinding("changed", path, oldState, newState))
		case oldTracked && !newTracked:
			result.Findings = append(result.Findings, fileFinding("became-untracked", path, oldState, newState))
		case !oldTracked && newTracked:
			result.Findings = append(result.Findings, fileFinding("became-tracked", path, oldState, newState))
		case !oldTracked && !newTracked && oldState.Size != newState.Size:
			result.Findings = append(result.Findings, fileFinding("size-changed", path, oldState, newState))
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

func tracked(state snapshot.FileState) bool { return state.Tracked || state.SHA256 != "" }

func fileFinding(change, path string, before, after snapshot.FileState) Finding {
	kind := before.Kind
	if kind == "" {
		kind = after.Kind
	}
	score, why := fileScore(kind)
	summary := fmt.Sprintf("%s %s", path, humanFileChange(change))
	if change == "became-untracked" && after.Reason == "size-limit" {
		summary = fmt.Sprintf("%s exceeded the content-hash size limit", path)
		why += "; the file is still present and is not reported as removed"
	} else if change == "size-changed" {
		summary = fmt.Sprintf("%s changed size without content hashing", path)
		why += "; only metadata is available because the file exceeds the hash limit"
	}
	return Finding{
		Category: fileCategory(kind), Change: change, Subject: path,
		Before: fileDescription(before), After: fileDescription(after), Summary: summary, Score: score, Why: why,
	}
}

func humanFileChange(change string) string {
	switch change {
	case "became-untracked":
		return "is now present but not content-hashed"
	case "became-tracked":
		return "is now content-hashed"
	case "size-changed":
		return "changed size"
	default:
		return change
	}
}

func fileDescription(state snapshot.FileState) string {
	if state.Kind == "" && state.Size == 0 && state.SHA256 == "" {
		return ""
	}
	if !tracked(state) {
		reason := state.Reason
		if reason == "" {
			reason = "content not hashed"
		}
		return fmt.Sprintf("present, %d bytes, untracked (%s)", state.Size, reason)
	}
	digest := state.SHA256
	if len(digest) > 12 {
		digest = digest[:12]
	}
	return fmt.Sprintf("sha256:%s, %d bytes", digest, state.Size)
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

func fileScore(kind string) (int, string) {
	switch kind {
	case "dependency":
		return 96, "dependency manifests and lockfiles can change the code that executes without an obvious source edit"
	case "migration":
		return 94, "migration files can alter persistent application state"
	case "config":
		return 90, "project configuration changed since the command last passed"
	case "source":
		return 86, "executable source changed since the command last passed"
	case "test":
		return 72, "test code changed, which can change the observed result"
	default:
		return 50, "a tracked project file changed"
	}
}

func compareEnvironment(result *Result, before, after *snapshot.Snapshot, context projectRelevance) {
	if !before.Complete["environment"] || !after.Complete["environment"] {
		result.Skipped = append(result.Skipped, "environment")
		return
	}
	for name, oldState := range before.Environment {
		newState, found := after.Environment[name]
		if !found {
			result.Findings = append(result.Findings, environmentFinding("removed", name, oldState, snapshot.EnvState{}, context))
			continue
		}
		if oldState.SHA256 != newState.SHA256 {
			result.Findings = append(result.Findings, environmentFinding("changed", name, oldState, newState, context))
		}
	}
	for name, state := range after.Environment {
		if _, found := before.Environment[name]; !found {
			result.Findings = append(result.Findings, environmentFinding("added", name, snapshot.EnvState{}, state, context))
		}
	}
}

func environmentFinding(change, name string, before, after snapshot.EnvState, context projectRelevance) Finding {
	sensitive := before.Sensitivity != "safe" || after.Sensitivity != "safe"
	if before.SHA256 == "" {
		sensitive = after.Sensitivity != "safe"
	}
	if after.SHA256 == "" {
		sensitive = before.Sensitivity != "safe"
	}
	score, why := environmentScore(name, sensitive, context)
	summary := fmt.Sprintf("%s %s", name, change)
	if before.SHA256 == "" && after.Sensitivity == "safe" {
		summary += fmt.Sprintf(" (%s)", truncate(after.Value, 40))
	} else if after.SHA256 == "" && before.Sensitivity == "safe" {
		summary += fmt.Sprintf(" (was %s)", truncate(before.Value, 40))
	} else if before.Sensitivity == "safe" && after.Sensitivity == "safe" && before.SHA256 != "" && after.SHA256 != "" {
		summary += fmt.Sprintf(" (%s -> %s)", truncate(before.Value, 40), truncate(after.Value, 40))
	}
	return Finding{
		Category: "environment", Change: change, Subject: name,
		Before: envDescription(before), After: envDescription(after),
		Summary: summary, Score: score, Why: why, Sensitive: sensitive,
	}
}

func envDescription(state snapshot.EnvState) string {
	if state.SHA256 == "" {
		return ""
	}
	if state.Sensitivity != "safe" || state.Value == "" {
		if state.Sensitivity == "safe" {
			return "<empty>"
		}
		return "<value hidden>"
	}
	return truncate(state.Value, 120)
}

func environmentScore(name string, sensitive bool, context projectRelevance) (int, string) {
	upper := strings.ToUpper(name)
	if reference, found := context.environment[upper]; found {
		return 93, fmt.Sprintf("%s references %s, so its value is repository-relevant", reference.Source, upper)
	}
	if sensitive {
		return 82, "a hidden environment value changed; no plaintext was stored"
	}
	important := []string{"CONFIG", "DATABASE", "DB_", "ENV", "HOST", "JAVA", "NODE", "POOL", "PORT", "PROFILE", "PYTHON", "REDIS", "REGION", "TIMEZONE", "TZ", "URL"}
	for _, fragment := range important {
		if strings.Contains(upper, fragment) {
			return 84, "the variable name looks runtime- or project-relevant"
		}
	}
	return 62, "an inherited environment value changed"
}

func compareRuntimes(result *Result, before, after *snapshot.Snapshot, context projectRelevance) {
	if !before.Complete["runtimes"] || !after.Complete["runtimes"] {
		result.Skipped = append(result.Skipped, "runtimes")
		return
	}
	compareSimpleMaps(before.Runtimes, after.Runtimes, func(change, name string, oldState, newState snapshot.RuntimeState) {
		score, why := runtimeScore(name, context)
		summary := fmt.Sprintf("%s runtime %s", name, change)
		if oldState.Version != "" && newState.Version != "" {
			summary += fmt.Sprintf(" (%s -> %s)", truncate(oldState.Version, 50), truncate(newState.Version, 50))
		}
		result.Findings = append(result.Findings, Finding{
			Category: "runtimes", Change: change, Subject: name, Before: oldState.Version, After: newState.Version,
			Summary: summary, Score: score, Why: why,
		})
	})
}

func runtimeScore(name string, context projectRelevance) (int, string) {
	uses := func(values map[string]struct{}, names ...string) bool {
		for _, name := range names {
			if _, found := values[name]; found {
				return true
			}
		}
		return false
	}
	switch name {
	case "Java":
		if uses(context.languages, "Java", "Kotlin", "Scala") || uses(context.buildSystems, "Maven", "Gradle") {
			return 95, "the repository uses Java/JVM source or a Maven/Gradle build"
		}
	case "Node.js":
		if uses(context.languages, "JavaScript", "TypeScript") || uses(context.buildSystems, "Node/npm") {
			return 95, "the repository contains package.json or JavaScript/TypeScript source"
		}
	case "Go":
		if uses(context.languages, "Go") || uses(context.buildSystems, "Go modules") {
			return 95, "the repository contains Go source or go.mod"
		}
	case "Python":
		if uses(context.languages, "Python") || uses(context.buildSystems, "Python/pyproject", "Python/pip") {
			return 95, "the repository contains Python source or Python dependency metadata"
		}
	case "Rust":
		if uses(context.languages, "Rust") || uses(context.buildSystems, "Cargo") {
			return 95, "the repository contains Rust source or Cargo metadata"
		}
	case "Docker":
		if len(context.services) > 0 {
			return 92, "the repository defines container services"
		}
	}
	return 88, "runtime version or availability changed"
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
			Category: "git", Change: "changed", Subject: "branch", Before: before.Git.Branch, After: after.Git.Branch,
			Summary: "Git branch changed", Score: 92, Why: "a branch switch can replace many project inputs at once",
		})
	}
	if before.Git.Commit != after.Git.Commit {
		result.Findings = append(result.Findings, Finding{
			Category: "git", Change: "changed", Subject: "commit", Before: shortCommit(before.Git.Commit), After: shortCommit(after.Git.Commit),
			Summary: "Git commit changed", Score: 68, Why: "the checked-out repository revision changed",
		})
	}
}

func comparePorts(result *Result, before, after *snapshot.Snapshot, context projectRelevance) {
	if !before.Complete["ports"] || !after.Complete["ports"] {
		result.Skipped = append(result.Skipped, "listening ports")
		return
	}
	for key, oldState := range before.Ports {
		newState, found := after.Ports[key]
		if !found {
			result.Findings = append(result.Findings, portFinding("disappeared", key, oldState, snapshot.PortState{}, context))
			continue
		}
		if oldState.Owner != newState.Owner && oldState.Owner != "" && newState.Owner != "" {
			result.Findings = append(result.Findings, portFinding("owner-changed", key, oldState, newState, context))
		}
	}
	for key, state := range after.Ports {
		if _, found := before.Ports[key]; !found {
			result.Findings = append(result.Findings, portFinding("appeared", key, snapshot.PortState{}, state, context))
		}
	}
}

var commonDevelopmentPorts = map[int]struct{}{
	3000: {}, 3001: {}, 3306: {}, 4200: {}, 5000: {}, 5173: {}, 5432: {}, 6379: {},
	8000: {}, 8080: {}, 8081: {}, 8090: {}, 9000: {}, 9200: {}, 27017: {},
}

func portFinding(change, key string, before, after snapshot.PortState, context projectRelevance) Finding {
	state := before
	if state.Port == 0 {
		state = after
	}
	score := 55
	why := "a listening TCP port changed state"
	label := "TCP"
	if reference, found := context.ports[state.Port]; found {
		score = 97
		service := context.services[reference.Service]
		if service.Type != "" {
			label = service.Type
		} else if reference.Service != "" {
			label = reference.Service
		}
		if reference.Service != "" {
			why = fmt.Sprintf("%s defines service %s on port %d, and the listener state changed since the command passed", reference.Source, reference.Service, state.Port)
		} else {
			why = fmt.Sprintf("%s references port %d, making it repository-relevant", reference.Source, state.Port)
		}
	} else if _, common := commonDevelopmentPorts[state.Port]; common {
		score = 82
		why = "this is a commonly used development or database port"
	}
	summaryChange := change
	if change == "owner-changed" {
		summaryChange = "owner changed"
	}
	return Finding{
		Category: "ports", Change: change, Subject: key, Before: portDescription(before), After: portDescription(after),
		Summary: fmt.Sprintf("%s port %d %s", label, state.Port, summaryChange), Score: score, Why: why,
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
		result.Skipped = append(result.Skipped, "containers")
		return
	}
	compareSimpleMaps(before.Containers, after.Containers, func(change, name string, oldState, newState snapshot.ContainerState) {
		result.Findings = append(result.Findings, Finding{
			Category: "containers", Change: change, Subject: name,
			Before: containerDescription(oldState), After: containerDescription(newState),
			Summary: fmt.Sprintf("container %s %s", name, change), Score: 78, Why: "container image or running state changed",
		})
	})
}

func containerDescription(state snapshot.ContainerState) string {
	if state.Image == "" && state.State == "" {
		return ""
	}
	return strings.TrimSpace(state.Image + " (" + state.State + ")")
}

func buildRelevance(values ...*snapshot.Snapshot) projectRelevance {
	result := projectRelevance{
		languages: map[string]struct{}{}, buildSystems: map[string]struct{}{},
		environment: map[string]snapshot.EnvReference{}, ports: map[int]snapshot.PortRef{}, services: map[string]snapshot.ServiceRef{},
	}
	for _, value := range values {
		for _, language := range value.Project.Languages {
			result.languages[language] = struct{}{}
		}
		for _, buildSystem := range value.Project.BuildSystems {
			result.buildSystems[buildSystem] = struct{}{}
		}
		for _, reference := range value.Project.ReferencedEnvironment {
			if _, found := result.environment[strings.ToUpper(reference.Name)]; !found {
				result.environment[strings.ToUpper(reference.Name)] = reference
			}
		}
		for _, reference := range value.Project.ReferencedPorts {
			if _, found := result.ports[reference.Port]; !found {
				result.ports[reference.Port] = reference
			}
		}
		for _, service := range value.Project.Services {
			if _, found := result.services[service.Name]; !found {
				result.services[service.Name] = service
			}
		}
	}
	return result
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

func uniqueSorted(values []string) []string {
	set := make(map[string]struct{}, len(values))
	for _, value := range values {
		set[value] = struct{}{}
	}
	result := make([]string, 0, len(set))
	for value := range set {
		result = append(result, value)
	}
	sort.Strings(result)
	return result
}
