package compare

import (
	"fmt"
	"testing"
	"time"

	"github.com/local-first/what-changed/internal/snapshot"
)

func TestSnapshotsRanksRelevantChangesAndRedactsSecrets(t *testing.T) {
	baseline := testSnapshot()
	current := testSnapshot()
	baseline.Files["package-lock.json"] = snapshot.FileState{SHA256: "old-lock", Kind: "dependency", Size: 10}
	current.Files["package-lock.json"] = snapshot.FileState{SHA256: "new-lock", Kind: "dependency", Size: 11}
	baseline.Files["src/main.go"] = snapshot.FileState{SHA256: "old-code", Kind: "source", Size: 20}
	current.Files["src/main.go"] = snapshot.FileState{SHA256: "new-code", Kind: "source", Size: 21}
	baseline.Environment["OPENAI_API_KEY"] = snapshot.EnvState{SHA256: "secret-one", Redacted: true}
	current.Environment["OPENAI_API_KEY"] = snapshot.EnvState{SHA256: "secret-two", Redacted: true}
	current.Ports["tcp:8080"] = snapshot.PortState{Protocol: "tcp", Address: "127.0.0.1", Port: 8080}

	result := Snapshots(baseline, current)
	if len(result.Findings) != 4 {
		t.Fatalf("got %d findings, want 4: %#v", len(result.Findings), result.Findings)
	}
	if got := result.Findings[0]; got.Subject != "package-lock.json" || got.Score != 96 {
		t.Fatalf("top finding=%#v, want dependency score 96", got)
	}
	var secret *Finding
	for index := range result.Findings {
		if result.Findings[index].Subject == "OPENAI_API_KEY" {
			secret = &result.Findings[index]
		}
	}
	if secret == nil || !secret.Sensitive {
		t.Fatalf("secret finding missing or not sensitive: %#v", secret)
	}
	if secret.Before != "<redacted>" || secret.After != "<redacted>" {
		t.Fatalf("secret values leaked: %#v", secret)
	}
}

func TestSnapshotsAvoidsAdditionsFromPartialBaseline(t *testing.T) {
	baseline := testSnapshot()
	current := testSnapshot()
	baseline.Complete["files"] = false
	current.Files["src/new.go"] = snapshot.FileState{SHA256: "new", Kind: "source"}

	result := Snapshots(baseline, current)
	if len(result.Findings) != 0 {
		t.Fatalf("partial baseline produced false addition: %#v", result.Findings)
	}
	if len(result.Skipped) == 0 {
		t.Fatal("expected partial scan disclosure")
	}
}

func BenchmarkCompareTenThousandFiles(b *testing.B) {
	baseline := testSnapshot()
	current := testSnapshot()
	for index := 0; index < 10_000; index++ {
		path := fmt.Sprintf("src/file-%05d.go", index)
		digest := fmt.Sprintf("digest-%05d", index)
		baseline.Files[path] = snapshot.FileState{SHA256: digest, Kind: "source", Size: 100}
		current.Files[path] = snapshot.FileState{SHA256: digest, Kind: "source", Size: 100}
	}
	for index := 0; index < 100; index++ {
		path := fmt.Sprintf("src/file-%05d.go", index*10)
		state := current.Files[path]
		state.SHA256 = "changed-" + state.SHA256
		current.Files[path] = state
	}
	b.ResetTimer()
	for index := 0; index < b.N; index++ {
		result := Snapshots(baseline, current)
		if len(result.Findings) != 100 {
			b.Fatalf("got %d findings", len(result.Findings))
		}
	}
}

func testSnapshot() *snapshot.Snapshot {
	return &snapshot.Snapshot{
		SchemaVersion: snapshot.SchemaVersion,
		Label:         "test",
		CapturedAt:    time.Unix(1_700_000_000, 0).UTC(),
		Root:          "/project",
		Files:         map[string]snapshot.FileState{},
		Environment:   map[string]snapshot.EnvState{},
		Runtimes:      map[string]snapshot.RuntimeState{},
		Ports:         map[string]snapshot.PortState{},
		Containers:    map[string]snapshot.ContainerState{},
		Complete: map[string]bool{
			"files": true, "environment": true, "runtimes": true,
			"git": false, "ports": true, "containers": false,
		},
	}
}
