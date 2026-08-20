package render

import (
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"time"
	"unicode"

	"github.com/local-first/what-changed/internal/compare"
)

func Text(writer io.Writer, result compare.Result, limit int, now time.Time) error {
	if limit < 0 {
		limit = 0
	}
	age := humanAge(now.Sub(result.Baseline.CapturedAt))
	header := fmt.Sprintf("Since checkpoint %q (%s", result.Baseline.Label, age)
	if command := formatCommand(result.Baseline.Trigger.Command); command != "" {
		header += " after " + command
	}
	if note := strings.TrimSpace(result.Baseline.Trigger.Note); note != "" {
		header += "; note: " + truncate(note, 100)
	}
	header += "):"
	if _, err := fmt.Fprintln(writer, header); err != nil {
		return err
	}
	if len(result.Findings) == 0 {
		if _, err := fmt.Fprintln(writer, "No meaningful tracked changes found."); err != nil {
			return err
		}
		return writeSkipped(writer, result.Skipped)
	}
	if _, err := fmt.Fprintf(writer, "%d meaningful change%s found.\n\n", len(result.Findings), plural(len(result.Findings))); err != nil {
		return err
	}
	shown := len(result.Findings)
	if limit > 0 && shown > limit {
		shown = limit
	}
	for index, finding := range result.Findings[:shown] {
		if _, err := fmt.Fprintf(writer, "%d. [%d] %s — %s\n", index+1, finding.Score, title(finding.Category), finding.Summary); err != nil {
			return err
		}
		if finding.Before != "" || finding.After != "" {
			before, after := finding.Before, finding.After
			if before == "" {
				before = "<absent>"
			}
			if after == "" {
				after = "<absent>"
			}
			if _, err := fmt.Fprintf(writer, "   %s -> %s\n", before, after); err != nil {
				return err
			}
		}
		if _, err := fmt.Fprintf(writer, "   Why: %s.\n", sentence(finding.Why)); err != nil {
			return err
		}
	}
	if shown < len(result.Findings) {
		if _, err := fmt.Fprintf(writer, "\n%d more change%s hidden; use --limit 0 to show all.\n", len(result.Findings)-shown, plural(len(result.Findings)-shown)); err != nil {
			return err
		}
	}
	return writeSkipped(writer, result.Skipped)
}

func JSON(writer io.Writer, value any) error {
	encoder := json.NewEncoder(writer)
	encoder.SetIndent("", "  ")
	encoder.SetEscapeHTML(false)
	return encoder.Encode(value)
}

func writeSkipped(writer io.Writer, skipped []string) error {
	if len(skipped) == 0 {
		return nil
	}
	_, err := fmt.Fprintf(writer, "\nNot compared: %s.\n", strings.Join(skipped, ", "))
	return err
}

func formatCommand(command []string) string {
	if len(command) == 0 {
		return ""
	}
	parts := make([]string, 0, len(command))
	for _, argument := range command {
		if strings.ContainsAny(argument, " \t\"'") {
			parts = append(parts, fmt.Sprintf("%q", argument))
		} else {
			parts = append(parts, argument)
		}
	}
	return "`" + strings.Join(parts, " ") + "`"
}

func humanAge(duration time.Duration) string {
	if duration < 0 {
		duration = 0
	}
	switch {
	case duration < time.Minute:
		return "just now"
	case duration < time.Hour:
		minutes := int(duration.Minutes())
		return fmt.Sprintf("%d minute%s ago", minutes, plural(minutes))
	case duration < 24*time.Hour:
		hours := int(duration.Hours())
		return fmt.Sprintf("%d hour%s ago", hours, plural(hours))
	default:
		days := int(duration.Hours() / 24)
		return fmt.Sprintf("%d day%s ago", days, plural(days))
	}
}

func plural(count int) string {
	if count == 1 {
		return ""
	}
	return "s"
}

func title(value string) string {
	if value == "" {
		return value
	}
	runes := []rune(value)
	runes[0] = unicode.ToUpper(runes[0])
	return string(runes)
}

func sentence(value string) string {
	value = strings.TrimSpace(value)
	return strings.TrimSuffix(value, ".")
}

func truncate(value string, limit int) string {
	if len(value) <= limit {
		return value
	}
	return value[:limit-1] + "…"
}
