package snapshot

import (
	"context"
	"fmt"
	"testing"
)

func TestProjectContextDetectsJavaNodeEnvironmentAndDockerServices(t *testing.T) {
	root := t.TempDir()
	writeTestFile(t, root, "pom.xml", `<project><dependencies><dependency><artifactId>postgresql</artifactId></dependency></dependencies></project>`)
	writeTestFile(t, root, "src/App.java", `class App {}`)
	writeTestFile(t, root, "package.json", `{"dependencies":{"express":"1.0.0"},"devDependencies":{"vitest":"1.0.0"}}`)
	writeTestFile(t, root, "config/app.yaml", "redis:\n  host: ${REDIS_HOST}\n  port: ${REDIS_PORT}\n")
	writeTestFile(t, root, "docker-compose.yml", "services:\n  postgres:\n    image: postgres:16\n    ports:\n      - \"5432:5432\"\n")
	files, _, complete, diagnostics := captureFiles(context.Background(), root, nil)
	if !complete {
		t.Fatalf("file scan incomplete: %v", diagnostics)
	}
	project, complete, diagnostics := captureProjectContext(context.Background(), root, files)
	if !complete {
		t.Fatalf("context incomplete: %v", diagnostics)
	}
	assertContains(t, project.Languages, "Java")
	assertContains(t, project.BuildSystems, "Maven")
	assertContains(t, project.BuildSystems, "Node/npm")
	assertContains(t, project.Dependencies, "postgresql")
	assertContains(t, project.Dependencies, "express")
	if !hasEnvReference(project, "REDIS_HOST", "config/app.yaml") || !hasEnvReference(project, "REDIS_PORT", "config/app.yaml") {
		t.Fatalf("missing environment references: %#v", project.ReferencedEnvironment)
	}
	if !hasPortReference(project, 5432, "postgres") {
		t.Fatalf("missing PostgreSQL port reference: %#v", project.ReferencedPorts)
	}
	if len(project.Services) != 1 || project.Services[0].Type != "PostgreSQL" {
		t.Fatalf("unexpected services: %#v", project.Services)
	}
}

func TestProjectContextFailureIsPartialAndIsolated(t *testing.T) {
	root := t.TempDir()
	files := map[string]FileState{
		"missing.yaml": {Size: 20, Kind: "config", Tracked: true, SHA256: "digest"},
		"src/main.go":  {Size: 10, Kind: "source", Tracked: true, SHA256: "digest"},
	}
	project, complete, diagnostics := captureProjectContext(context.Background(), root, files)
	if complete {
		t.Fatal("missing config should make project-context detector partial")
	}
	if len(diagnostics) == 0 || len(project.Languages) != 1 || project.Languages[0] != "Go" {
		t.Fatalf("partial context lost useful results: project=%#v diagnostics=%#v", project, diagnostics)
	}
}

func BenchmarkProjectContextTenThousandFiles(b *testing.B) {
	root := b.TempDir()
	writeBenchmarkFile(b, root, "package.json", `{"dependencies":{"express":"1"}}`)
	writeBenchmarkFile(b, root, "docker-compose.yml", "services:\n  redis:\n    image: redis:7\n    ports:\n      - \"6379:6379\"\n")
	files := map[string]FileState{
		"package.json":       {Size: 31, Kind: "dependency", Tracked: true},
		"docker-compose.yml": {Size: 72, Kind: "config", Tracked: true},
	}
	for index := 0; index < 10_000; index++ {
		files[fmt.Sprintf("src/file-%05d.go", index)] = FileState{Size: 100, Kind: "source", Tracked: true}
	}
	b.ResetTimer()
	for index := 0; index < b.N; index++ {
		project, complete, _ := captureProjectContext(context.Background(), root, files)
		if !complete || len(project.Languages) != 1 {
			b.Fatal("unexpected context result")
		}
	}
}

func assertContains(t *testing.T, values []string, want string) {
	t.Helper()
	for _, value := range values {
		if value == want {
			return
		}
	}
	t.Fatalf("%q not found in %#v", want, values)
}

func hasEnvReference(project ProjectContext, name, source string) bool {
	for _, reference := range project.ReferencedEnvironment {
		if reference.Name == name && reference.Source == source {
			return true
		}
	}
	return false
}

func hasPortReference(project ProjectContext, port int, service string) bool {
	for _, reference := range project.ReferencedPorts {
		if reference.Port == port && reference.Service == service {
			return true
		}
	}
	return false
}
