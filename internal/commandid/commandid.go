package commandid

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"path/filepath"
	"runtime"
	"strings"
	"unicode"
)

func Identity(root string, command []string) (string, string, error) {
	if len(command) == 0 || strings.TrimSpace(command[0]) == "" {
		return "", "", errors.New("command is empty")
	}
	normalizedRoot, err := NormalizeRoot(root)
	if err != nil {
		return "", "", err
	}
	normalizedCommand := make([]string, len(command))
	copy(normalizedCommand, command)
	normalizedCommand[0] = normalizeExecutable(normalizedCommand[0])

	hash := sha256.New()
	writePart := func(value string) {
		hash.Write([]byte{byte(len(value) >> 24), byte(len(value) >> 16), byte(len(value) >> 8), byte(len(value))})
		hash.Write([]byte(value))
	}
	writePart(normalizedRoot)
	for _, part := range normalizedCommand {
		writePart(part)
	}
	digest := hex.EncodeToString(hash.Sum(nil))[:8]
	return slug(normalizedCommand) + "-" + digest, normalizedRoot, nil
}

func NormalizeRoot(root string) (string, error) {
	absolute, err := filepath.Abs(root)
	if err != nil {
		return "", err
	}
	clean := filepath.Clean(absolute)
	if resolved, err := filepath.EvalSymlinks(clean); err == nil {
		clean = filepath.Clean(resolved)
	}
	clean = filepath.ToSlash(clean)
	if runtime.GOOS == "windows" {
		clean = strings.ToLower(clean)
	}
	return strings.TrimSuffix(clean, "/"), nil
}

func Display(command []string) string {
	parts := make([]string, 0, len(command))
	for _, argument := range command {
		if strings.ContainsAny(argument, " \t\"'") {
			parts = append(parts, "\""+strings.ReplaceAll(argument, "\"", "\\\"")+"\"")
		} else {
			parts = append(parts, argument)
		}
	}
	return strings.Join(parts, " ")
}

func normalizeExecutable(executable string) string {
	clean := filepath.Clean(executable)
	if runtime.GOOS == "windows" {
		clean = strings.ToLower(clean)
	}
	return filepath.ToSlash(clean)
}

func slug(command []string) string {
	words := make([]string, 0, 4)
	for index, part := range command {
		if index == 0 {
			part = filepath.Base(filepath.FromSlash(part))
			part = strings.TrimSuffix(strings.TrimSuffix(strings.TrimSuffix(strings.ToLower(part), ".exe"), ".cmd"), ".bat")
		}
		var word strings.Builder
		flush := func() {
			if word.Len() > 0 && len(words) < 4 {
				words = append(words, word.String())
				word.Reset()
			}
		}
		for _, char := range strings.ToLower(part) {
			if unicode.IsLetter(char) || unicode.IsDigit(char) {
				word.WriteRune(char)
			} else {
				flush()
			}
			if len(words) == 4 {
				break
			}
		}
		flush()
		if len(words) == 4 {
			break
		}
	}
	if len(words) == 0 {
		return "command"
	}
	value := strings.Join(words, "-")
	if len(value) > 32 {
		value = strings.Trim(value[:32], "-")
	}
	return value
}
