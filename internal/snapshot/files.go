package snapshot

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

const (
	maxTrackedFileSize = 5 << 20
	maxTrackedFiles    = 50_000
)

var ignoredDirectories = map[string]struct{}{
	".git": {}, ".what-changed": {}, ".gradle": {}, ".idea": {}, ".next": {},
	".cache": {}, "node_modules": {}, "vendor": {}, "target": {}, "build": {},
	"dist": {}, "out": {}, "coverage": {}, "__pycache__": {},
}

var dependencyNames = map[string]struct{}{
	"go.mod": {}, "go.sum": {}, "package.json": {}, "package-lock.json": {},
	"npm-shrinkwrap.json": {}, "pnpm-lock.yaml": {}, "yarn.lock": {}, "bun.lock": {},
	"bun.lockb": {}, "pom.xml": {}, "build.gradle": {}, "build.gradle.kts": {},
	"gradle.lockfile": {}, "cargo.toml": {}, "cargo.lock": {}, "gemfile": {},
	"gemfile.lock": {}, "requirements.txt": {}, "poetry.lock": {}, "pyproject.toml": {},
	"pipfile": {}, "pipfile.lock": {}, "composer.json": {}, "composer.lock": {},
	"packages.lock.json": {}, "paket.lock": {},
}

var sourceExtensions = map[string]struct{}{
	".c": {}, ".cc": {}, ".cpp": {}, ".cs": {}, ".css": {}, ".dart": {},
	".go": {}, ".h": {}, ".hpp": {}, ".html": {}, ".java": {}, ".js": {},
	".jsx": {}, ".kt": {}, ".kts": {}, ".lua": {}, ".php": {}, ".py": {},
	".rb": {}, ".rs": {}, ".scala": {}, ".scss": {}, ".sh": {}, ".sql": {},
	".swift": {}, ".ts": {}, ".tsx": {}, ".vue": {}, ".zig": {},
}

var configExtensions = map[string]struct{}{
	".conf": {}, ".config": {}, ".env": {}, ".ini": {}, ".json": {}, ".properties": {},
	".toml": {}, ".xml": {}, ".yaml": {}, ".yml": {},
}

func captureFiles(ctx context.Context, root string) (map[string]FileState, Stats, bool, []Diagnostic) {
	files := make(map[string]FileState)
	stats := Stats{}
	complete := true
	diagnostics := make([]Diagnostic, 0)

	err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, walkErr error) error {
		if err := ctx.Err(); err != nil {
			return err
		}
		if walkErr != nil {
			complete = false
			if len(diagnostics) < 10 {
				diagnostics = append(diagnostics, Diagnostic{Detector: "files", Severity: "warning", Message: walkErr.Error()})
			}
			if entry != nil && entry.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		if path == root {
			return nil
		}
		name := strings.ToLower(entry.Name())
		if entry.IsDir() {
			if _, ignored := ignoredDirectories[name]; ignored {
				stats.FilesSkipped++
				return filepath.SkipDir
			}
			return nil
		}
		if entry.Type()&os.ModeSymlink != 0 || !entry.Type().IsRegular() {
			stats.FilesSkipped++
			return nil
		}
		if len(files) >= maxTrackedFiles {
			complete = false
			diagnostics = append(diagnostics, Diagnostic{
				Detector: "files", Severity: "warning",
				Message: fmt.Sprintf("file limit reached (%d); scan is partial", maxTrackedFiles),
			})
			return filepath.SkipAll
		}

		info, err := entry.Info()
		if err != nil {
			complete = false
			return nil
		}
		if info.Size() > maxTrackedFileSize {
			stats.FilesSkippedLarge++
			return nil
		}
		digest, err := hashFile(path)
		if err != nil {
			complete = false
			if len(diagnostics) < 10 {
				diagnostics = append(diagnostics, Diagnostic{Detector: "files", Severity: "warning", Message: err.Error()})
			}
			return nil
		}
		relative, err := filepath.Rel(root, path)
		if err != nil {
			complete = false
			return nil
		}
		relative = filepath.ToSlash(relative)
		files[relative] = FileState{SHA256: digest, Size: info.Size(), Kind: classifyFile(relative)}
		stats.FilesHashed++
		return nil
	})
	if err != nil && !errors.Is(err, context.Canceled) && !errors.Is(err, context.DeadlineExceeded) {
		complete = false
		diagnostics = append(diagnostics, Diagnostic{Detector: "files", Severity: "warning", Message: err.Error()})
	}
	return files, stats, complete && err == nil, diagnostics
}

func hashFile(path string) (string, error) {
	file, err := os.Open(path)
	if err != nil {
		return "", fmt.Errorf("open %s: %w", path, err)
	}
	defer file.Close()
	hash := sha256.New()
	if _, err := io.Copy(hash, file); err != nil {
		return "", fmt.Errorf("hash %s: %w", path, err)
	}
	return hex.EncodeToString(hash.Sum(nil)), nil
}

func classifyFile(path string) string {
	lowerPath := strings.ToLower(filepath.ToSlash(path))
	base := filepath.Base(lowerPath)
	if _, ok := dependencyNames[base]; ok {
		return "dependency"
	}
	if strings.Contains(lowerPath, "/migration") || strings.Contains(lowerPath, "/migrations/") ||
		strings.HasPrefix(lowerPath, "migrations/") || strings.HasPrefix(base, "v") && strings.HasSuffix(base, ".sql") {
		return "migration"
	}
	extension := strings.ToLower(filepath.Ext(base))
	if strings.Contains(lowerPath, "/test/") || strings.Contains(lowerPath, "/tests/") ||
		strings.Contains(lowerPath, "_test.") || strings.Contains(lowerPath, ".test.") || strings.Contains(lowerPath, ".spec.") {
		return "test"
	}
	if base == "dockerfile" || base == "makefile" || base == ".env" || strings.HasPrefix(base, ".env.") {
		return "config"
	}
	if _, ok := configExtensions[extension]; ok {
		return "config"
	}
	if _, ok := sourceExtensions[extension]; ok {
		return "source"
	}
	return "other"
}
