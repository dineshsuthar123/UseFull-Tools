package snapshot

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestCaptureFilesHashesUsefulFilesAndIgnoresGeneratedDirectories(t *testing.T) {
	root := t.TempDir()
	writeTestFile(t, root, "src/main.go", "package main\n")
	writeTestFile(t, root, "package-lock.json", "{\"lockfileVersion\": 3}\n")
	writeTestFile(t, root, "db/migrations/V2__users.sql", "alter table users add active boolean;\n")
	writeTestFile(t, root, "node_modules/dependency/index.js", "generated\n")
	writeTestFile(t, root, ".what-changed/old.json", "{}\n")

	files, stats, complete, diagnostics := captureFiles(context.Background(), root)
	if !complete {
		t.Fatalf("expected complete scan, diagnostics=%v", diagnostics)
	}
	if stats.FilesHashed != 3 {
		t.Fatalf("hashed %d files, want 3", stats.FilesHashed)
	}
	if got := files["src/main.go"].Kind; got != "source" {
		t.Fatalf("main.go kind=%q, want source", got)
	}
	if got := files["package-lock.json"].Kind; got != "dependency" {
		t.Fatalf("package-lock kind=%q, want dependency", got)
	}
	if got := files["db/migrations/V2__users.sql"].Kind; got != "migration" {
		t.Fatalf("migration kind=%q, want migration", got)
	}
	if _, found := files["node_modules/dependency/index.js"]; found {
		t.Fatal("node_modules file should not be tracked")
	}
	if files["src/main.go"].SHA256 == "" {
		t.Fatal("expected a content digest")
	}
}

func TestClassifyFile(t *testing.T) {
	tests := map[string]string{
		"go.mod":                       "dependency",
		"frontend/package.json":        "dependency",
		"db/migrations/002_users.sql":  "migration",
		"src/payment_test.go":          "test",
		"src/PaymentService.java":      "source",
		"config/application-prod.yaml": "config",
		"README.md":                    "other",
	}
	for path, want := range tests {
		if got := classifyFile(path); got != want {
			t.Errorf("classifyFile(%q)=%q, want %q", path, got, want)
		}
	}
}

func TestCaptureHonorsCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err := Capture(ctx, Options{Root: t.TempDir(), Label: "canceled"})
	if err == nil {
		t.Fatal("Capture succeeded with a canceled context")
	}
}

func BenchmarkCaptureFiles1000(b *testing.B) {
	root := b.TempDir()
	for index := 0; index < 1000; index++ {
		path := filepath.Join("src", "pkg", string(rune('a'+index%26)), "file-"+itoa(index)+".go")
		writeBenchmarkFile(b, root, path, "package fixture\nvar Value = 42\n")
	}
	b.ResetTimer()
	for index := 0; index < b.N; index++ {
		_, _, complete, _ := captureFiles(context.Background(), root)
		if !complete {
			b.Fatal("scan unexpectedly incomplete")
		}
	}
}

func writeTestFile(t *testing.T, root, relative, content string) {
	t.Helper()
	path := filepath.Join(root, filepath.FromSlash(relative))
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
}

func writeBenchmarkFile(b *testing.B, root, relative, content string) {
	b.Helper()
	path := filepath.Join(root, filepath.FromSlash(relative))
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		b.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		b.Fatal(err)
	}
}

func itoa(value int) string {
	if value == 0 {
		return "0"
	}
	buffer := [20]byte{}
	position := len(buffer)
	for value > 0 {
		position--
		buffer[position] = byte('0' + value%10)
		value /= 10
	}
	return string(buffer[position:])
}
