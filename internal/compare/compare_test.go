package compare

import (
	"encoding/json"
	"fmt"
	"testing"
	"time"

	"github.com/dineshsuthar123/UseFull-Tools/internal/snapshot"
)

func TestSnapshotsRanksRelevantChangesAndHidesEnvironmentValues(t *testing.T) {
	baseline := testSnapshot()
	current := testSnapshot()
	baseline.Files["package-lock.json"] = trackedFile("old-lock", "dependency", 10)
	current.Files["package-lock.json"] = trackedFile("new-lock", "dependency", 11)
	baseline.Files["src/main.go"] = trackedFile("old-code", "source", 20)
	current.Files["src/main.go"] = trackedFile("new-code", "source", 21)
	baseline.Environment["OPENAI_API_KEY"] = snapshot.EnvState{SHA256: "secret-one", Sensitivity: "secret-name"}
	current.Environment["OPENAI_API_KEY"] = snapshot.EnvState{SHA256: "secret-two", Sensitivity: "secret-name"}
	current.Ports["tcp:8080"] = snapshot.PortState{Protocol: "tcp", Address: "127.0.0.1", Port: 8080}

	result := Snapshots(baseline, current)
	if len(result.Findings) != 4 {
		t.Fatalf("got %d findings, want 4: %#v", len(result.Findings), result.Findings)
	}
	if got := result.Findings[0]; got.Subject != "package-lock.json" || got.Score != 96 {
		t.Fatalf("top finding=%#v, want dependency score 96", got)
	}
	secret := findSubject(result.Findings, "OPENAI_API_KEY")
	if secret == nil || !secret.Sensitive || secret.Before != "<value hidden>" || secret.After != "<value hidden>" {
		t.Fatalf("secret finding leaked or missing: %#v", secret)
	}
}

func TestOversizedFilePresenceAndThresholdCrossing(t *testing.T) {
	baseline := testSnapshot()
	current := testSnapshot()
	large := snapshot.FileState{Size: 7 << 20, Kind: "other", Reason: "size-limit"}
	baseline.Files["large.bin"] = large
	current.Files["large.bin"] = large
	if result := Snapshots(baseline, current); len(result.Findings) != 0 {
		t.Fatalf("unchanged oversized file produced findings: %#v", result.Findings)
	}

	baseline.Files["crossing.bin"] = trackedFile("old", "other", 100)
	current.Files["crossing.bin"] = snapshot.FileState{Size: 7 << 20, Kind: "other", Reason: "size-limit"}
	result := Snapshots(baseline, current)
	finding := findSubject(result.Findings, "crossing.bin")
	if finding == nil || finding.Change != "became-untracked" || finding.Change == "removed" {
		t.Fatalf("threshold crossing misclassified: %#v", finding)
	}
}

func TestDeletedOversizedFileIsReportedAsRemoved(t *testing.T) {
	baseline := testSnapshot()
	current := testSnapshot()
	baseline.Files["large.bin"] = snapshot.FileState{Size: 7 << 20, Kind: "other", Reason: "size-limit"}
	result := Snapshots(baseline, current)
	finding := findSubject(result.Findings, "large.bin")
	if finding == nil || finding.Change != "removed" {
		t.Fatalf("deleted oversized file=%#v, want removed", finding)
	}
}

func TestSnapshotsAvoidsAdditionsFromPartialBaseline(t *testing.T) {
	baseline := testSnapshot()
	current := testSnapshot()
	baseline.Complete["files"] = false
	current.Files["src/new.go"] = trackedFile("new", "source", 10)
	result := Snapshots(baseline, current)
	if len(result.Findings) != 0 || len(result.Skipped) == 0 {
		t.Fatalf("partial baseline result=%#v", result)
	}
}

func TestRepositoryLanguageBoostsRuntimeRelevance(t *testing.T) {
	tests := []struct {
		name, runtime string
		project       snapshot.ProjectContext
		whyFragment   string
	}{
		{name: "Java", runtime: "Java", project: snapshot.ProjectContext{BuildSystems: []string{"Maven"}}, whyFragment: "Maven"},
		{name: "Node", runtime: "Node.js", project: snapshot.ProjectContext{BuildSystems: []string{"Node/npm"}}, whyFragment: "package.json"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			baseline, current := testSnapshot(), testSnapshot()
			baseline.Project, current.Project = test.project, test.project
			baseline.Runtimes[test.runtime] = snapshot.RuntimeState{Version: "old"}
			current.Runtimes[test.runtime] = snapshot.RuntimeState{Version: "new"}
			finding := findSubject(Snapshots(baseline, current).Findings, test.runtime)
			if finding == nil || finding.Score != 95 || !contains(finding.Why, test.whyFragment) {
				t.Fatalf("runtime finding=%#v", finding)
			}
		})
	}
}

func TestDockerServicePortGetsHighestContextScore(t *testing.T) {
	baseline, current := testSnapshot(), testSnapshot()
	project := snapshot.ProjectContext{
		Services:        []snapshot.ServiceRef{{Name: "postgres", Type: "PostgreSQL", Source: "docker-compose.yml"}},
		ReferencedPorts: []snapshot.PortRef{{Port: 5432, Service: "postgres", Source: "docker-compose.yml"}},
	}
	baseline.Project, current.Project = project, project
	baseline.Ports["tcp:5432"] = snapshot.PortState{Protocol: "tcp", Address: "0.0.0.0", Port: 5432}
	finding := findSubject(Snapshots(baseline, current).Findings, "tcp:5432")
	if finding == nil || finding.Score != 97 || finding.Summary != "PostgreSQL port 5432 disappeared" || !contains(finding.Why, "docker-compose.yml") {
		t.Fatalf("port finding=%#v", finding)
	}
}

func TestReferencedEnvironmentGetsContextBoostWithoutValues(t *testing.T) {
	baseline, current := testSnapshot(), testSnapshot()
	project := snapshot.ProjectContext{ReferencedEnvironment: []snapshot.EnvReference{{Name: "REDIS_HOST", Source: "config/app.yaml"}}}
	baseline.Project, current.Project = project, project
	baseline.Environment["REDIS_HOST"] = snapshot.EnvState{SHA256: "one", Sensitivity: "unknown"}
	current.Environment["REDIS_HOST"] = snapshot.EnvState{SHA256: "two", Sensitivity: "unknown"}
	finding := findSubject(Snapshots(baseline, current).Findings, "REDIS_HOST")
	if finding == nil || finding.Score != 93 || finding.Before != "<value hidden>" || !contains(finding.Why, "config/app.yaml") {
		t.Fatalf("environment finding=%#v", finding)
	}
}

func TestRankingAndJSONAreDeterministic(t *testing.T) {
	baseline, current := testSnapshot(), testSnapshot()
	for _, path := range []string{"z.go", "a.go", "m.go"} {
		baseline.Files[path] = trackedFile("old", "source", 10)
		current.Files[path] = trackedFile("new", "source", 10)
	}
	want, err := json.Marshal(Snapshots(baseline, current))
	if err != nil {
		t.Fatal(err)
	}
	for index := 0; index < 20; index++ {
		got, err := json.Marshal(Snapshots(baseline, current))
		if err != nil || string(got) != string(want) {
			t.Fatalf("iteration %d produced unstable JSON", index)
		}
	}
}

func BenchmarkCompareTenThousandFiles(b *testing.B) {
	baseline, current := testSnapshot(), testSnapshot()
	for index := 0; index < 10_000; index++ {
		path := fmt.Sprintf("src/file-%05d.go", index)
		digest := fmt.Sprintf("digest-%05d", index)
		baseline.Files[path] = trackedFile(digest, "source", 100)
		current.Files[path] = trackedFile(digest, "source", 100)
	}
	for index := 0; index < 100; index++ {
		path := fmt.Sprintf("src/file-%05d.go", index*10)
		state := current.Files[path]
		state.SHA256 = "changed-" + state.SHA256
		current.Files[path] = state
	}
	b.ResetTimer()
	for index := 0; index < b.N; index++ {
		if result := Snapshots(baseline, current); len(result.Findings) != 100 {
			b.Fatalf("got %d findings", len(result.Findings))
		}
	}
}

func testSnapshot() *snapshot.Snapshot {
	return &snapshot.Snapshot{
		SchemaVersion: snapshot.SchemaVersion, Label: "test", CapturedAt: time.Unix(1_700_000_000, 0).UTC(), Root: "/project",
		Files: map[string]snapshot.FileState{}, Environment: map[string]snapshot.EnvState{},
		Runtimes: map[string]snapshot.RuntimeState{}, Ports: map[string]snapshot.PortState{},
		Containers: map[string]snapshot.ContainerState{}, Complete: map[string]bool{
			"files": true, "environment": true, "runtimes": true, "git": false,
			"ports": true, "containers": true, "projectContext": true,
		},
	}
}

func trackedFile(digest, kind string, size int64) snapshot.FileState {
	return snapshot.FileState{SHA256: digest, Kind: kind, Size: size, Tracked: true}
}

func findSubject(findings []Finding, subject string) *Finding {
	for index := range findings {
		if findings[index].Subject == subject {
			return &findings[index]
		}
	}
	return nil
}

func contains(value, fragment string) bool {
	for index := 0; index+len(fragment) <= len(value); index++ {
		if value[index:index+len(fragment)] == fragment {
			return true
		}
	}
	return false
}
