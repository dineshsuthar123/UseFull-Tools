package store

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
	"unicode"

	"github.com/dineshsuthar123/UseFull-Tools/internal/commandid"
	"github.com/dineshsuthar123/UseFull-Tools/internal/snapshot"
)

var ErrNotFound = errors.New("checkpoint not found")

const directoryName = ".what-changed"

type Entry struct {
	Path     string             `json:"path"`
	Snapshot *snapshot.Snapshot `json:"snapshot"`
}

func Save(root string, value *snapshot.Snapshot) (string, error) {
	if value == nil {
		return "", errors.New("cannot save a nil checkpoint")
	}
	value.Label = normalizeLabel(value.Label)
	directory := filepath.Join(root, directoryName)
	if err := os.MkdirAll(directory, 0o700); err != nil {
		return "", fmt.Errorf("create checkpoint directory: %w", err)
	}
	temporary, err := os.CreateTemp(directory, ".checkpoint-*.tmp")
	if err != nil {
		return "", fmt.Errorf("create temporary checkpoint: %w", err)
	}
	temporaryPath := temporary.Name()
	keepTemporary := true
	defer func() {
		if keepTemporary {
			_ = os.Remove(temporaryPath)
		}
	}()
	if err := temporary.Chmod(0o600); err != nil {
		_ = temporary.Close()
		return "", fmt.Errorf("secure temporary checkpoint: %w", err)
	}
	encoder := json.NewEncoder(temporary)
	encoder.SetIndent("", "  ")
	encoder.SetEscapeHTML(false)
	if err := encoder.Encode(value); err != nil {
		_ = temporary.Close()
		return "", fmt.Errorf("encode checkpoint: %w", err)
	}
	if err := temporary.Sync(); err != nil {
		_ = temporary.Close()
		return "", fmt.Errorf("flush checkpoint: %w", err)
	}
	if err := temporary.Close(); err != nil {
		return "", fmt.Errorf("close checkpoint: %w", err)
	}

	stamp := value.CapturedAt.UTC().Format("20060102T150405.000000000Z")
	base := fmt.Sprintf("%s-%s.json", slug(value.Label), stamp)
	target := filepath.Join(directory, base)
	for suffix := 1; ; suffix++ {
		if _, err := os.Stat(target); errors.Is(err, fs.ErrNotExist) {
			break
		} else if err != nil {
			return "", fmt.Errorf("inspect checkpoint target: %w", err)
		}
		target = filepath.Join(directory, fmt.Sprintf("%s-%s-%d.json", slug(value.Label), stamp, suffix))
	}
	if err := os.Rename(temporaryPath, target); err != nil {
		return "", fmt.Errorf("commit checkpoint: %w", err)
	}
	keepTemporary = false
	return target, nil
}

func Load(root, label string) (*snapshot.Snapshot, string, error) {
	entries, err := List(root)
	if err != nil {
		return nil, "", err
	}
	wanted := strings.TrimSpace(label)
	for _, entry := range entries {
		if wanted == "" || entry.Snapshot.Label == wanted {
			return entry.Snapshot, entry.Path, nil
		}
	}
	if wanted == "" {
		wanted = "latest"
	}
	return nil, "", fmt.Errorf("%w: %s", ErrNotFound, wanted)
}

func LoadByCommandID(root, commandID string) (*snapshot.Snapshot, string, error) {
	entries, err := List(root)
	if err != nil {
		return nil, "", err
	}
	for _, entry := range entries {
		if entry.Snapshot.CommandID == commandID {
			return entry.Snapshot, entry.Path, nil
		}
	}
	return nil, "", fmt.Errorf("%w: command %s", ErrNotFound, commandID)
}

func List(root string) ([]Entry, error) {
	directory := filepath.Join(root, directoryName)
	directoryEntries, err := os.ReadDir(directory)
	if errors.Is(err, fs.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("read checkpoint directory: %w", err)
	}
	result := make([]Entry, 0, len(directoryEntries))
	for _, directoryEntry := range directoryEntries {
		if directoryEntry.IsDir() || !strings.HasSuffix(strings.ToLower(directoryEntry.Name()), ".json") {
			continue
		}
		path := filepath.Join(directory, directoryEntry.Name())
		value, err := read(path)
		if err != nil {
			continue
		}
		result = append(result, Entry{Path: path, Snapshot: value})
	}
	sort.Slice(result, func(i, j int) bool {
		if result[i].Snapshot.CapturedAt.Equal(result[j].Snapshot.CapturedAt) {
			return result[i].Path > result[j].Path
		}
		return result[i].Snapshot.CapturedAt.After(result[j].Snapshot.CapturedAt)
	})
	return result, nil
}

func read(path string) (*snapshot.Snapshot, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	decoder := json.NewDecoder(file)
	decoder.DisallowUnknownFields()
	var value snapshot.Snapshot
	if err := decoder.Decode(&value); err != nil {
		return nil, err
	}
	if value.SchemaVersion == 1 {
		snapshot.UpgradeV1(&value)
		if len(value.Trigger.Command) > 0 {
			if id, _, err := commandid.Identity(value.Root, value.Trigger.Command); err == nil {
				value.CommandID = id
			}
		}
	}
	if value.SchemaVersion != snapshot.SchemaVersion {
		return nil, fmt.Errorf("unsupported checkpoint schema %d", value.SchemaVersion)
	}
	return &value, nil
}

func normalizeLabel(label string) string {
	label = strings.TrimSpace(label)
	if label == "" {
		return "last-success"
	}
	if len(label) > 80 {
		return label[:80]
	}
	return label
}

func slug(label string) string {
	var result strings.Builder
	lastDash := false
	for _, char := range strings.ToLower(label) {
		if unicode.IsLetter(char) || unicode.IsDigit(char) {
			result.WriteRune(char)
			lastDash = false
		} else if !lastDash && result.Len() > 0 {
			result.WriteByte('-')
			lastDash = true
		}
	}
	value := strings.Trim(result.String(), "-")
	if value == "" {
		return fmt.Sprintf("checkpoint-%d", time.Now().UnixNano())
	}
	if len(value) > 48 {
		return strings.Trim(value[:48], "-")
	}
	return value
}
