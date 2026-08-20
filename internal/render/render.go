package render

import (
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"time"
	"unicode"

	"github.com/dineshsuthar123/UseFull-Tools/internal/commandid"
	"github.com/dineshsuthar123/UseFull-Tools/internal/compare"
)

type TextOptions struct {
	Limit   int
	Verbose bool
	Now     time.Time
}

func Text(writer io.Writer, result compare.Result, opts TextOptions) error {
	if opts.Limit < 0 {
		opts.Limit = 0
	}
	if opts.Now.IsZero() {
		opts.Now = time.Now()
	}
	age := humanAge(opts.Now.Sub(result.Baseline.CapturedAt))
	command := commandid.Display(result.Baseline.Trigger.Command)
	if result.Baseline.Trigger.Kind == "successful-command" && command != "" {
		if _, err := fmt.Fprintf(writer, "Since `%s` last passed %s:\n", command, age); err != nil {
			return err
		}
	} else {
		if _, err := fmt.Fprintf(writer, "Since checkpoint %q (%s):\n", result.Baseline.Label, age); err != nil {
			return err
		}
	}
	if opts.Verbose && strings.TrimSpace(result.Baseline.Trigger.Note) != "" {
		if _, err := fmt.Fprintf(writer, "Note: %s\n", truncate(strings.TrimSpace(result.Baseline.Trigger.Note), 160)); err != nil {
			return err
		}
	}
	if len(result.Findings) == 0 {
		if _, err := fmt.Fprintln(writer, "\nNo meaningful changes found."); err != nil {
			return err
		}
		return writeDetectorStatus(writer, result, opts.Verbose)
	}
	if _, err := fmt.Fprintln(writer, "\nLikely relevant changes:"); err != nil {
		return err
	}
	shown := len(result.Findings)
	if opts.Limit > 0 && shown > opts.Limit {
		shown = opts.Limit
	}
	for index, finding := range result.Findings[:shown] {
		if _, err := fmt.Fprintf(writer, "\n%d. [%d] %s\n", index+1, finding.Score, finding.Summary); err != nil {
			return err
		}
		if opts.Verbose {
			if finding.Before != "" || finding.After != "" {
				before, after := finding.Before, finding.After
				if before == "" {
					before = "<absent>"
				}
				if after == "" {
					after = "<absent>"
				}
				if _, err := fmt.Fprintf(writer, "   Fact: %s -> %s\n", before, after); err != nil {
					return err
				}
			}
			if _, err := fmt.Fprintf(writer, "   Why this is ranked highly: %s.\n", sentence(finding.Why)); err != nil {
				return err
			}
		} else {
			if _, err := fmt.Fprintf(writer, "   %s.\n", sentence(finding.Why)); err != nil {
				return err
			}
		}
	}
	if shown < len(result.Findings) {
		if _, err := fmt.Fprintf(writer, "\n%d more change%s hidden; use --limit 0 to show all.\n", len(result.Findings)-shown, plural(len(result.Findings)-shown)); err != nil {
			return err
		}
	}
	if _, err := fmt.Fprintf(writer, "\n%d meaningful change%s found.\n", len(result.Findings), plural(len(result.Findings))); err != nil {
		return err
	}
	return writeDetectorStatus(writer, result, opts.Verbose)
}

func JSON(writer io.Writer, value any) error {
	encoder := json.NewEncoder(writer)
	encoder.SetIndent("", "  ")
	encoder.SetEscapeHTML(false)
	return encoder.Encode(value)
}

func writeDetectorStatus(writer io.Writer, result compare.Result, verbose bool) error {
	diagnosticKeys := make(map[string]struct{})
	for _, diagnostic := range result.Diagnostics {
		diagnosticKeys[statusKey(diagnostic.Detector)] = struct{}{}
	}
	remainingSkipped := make([]string, 0, len(result.Skipped))
	for _, skipped := range result.Skipped {
		if _, duplicate := diagnosticKeys[statusKey(skipped)]; !duplicate {
			remainingSkipped = append(remainingSkipped, skipped)
		}
	}
	if !verbose {
		keys := make(map[string]struct{})
		for _, diagnostic := range result.Diagnostics {
			keys[statusKey(diagnostic.Detector)] = struct{}{}
		}
		for _, skipped := range remainingSkipped {
			keys[statusKey(skipped)] = struct{}{}
		}
		count := len(keys)
		if count == 0 {
			return nil
		}
		if _, err := fmt.Fprintf(writer, "\n%d detector note%s; use --verbose for details.\n", count, plural(count)); err != nil {
			return err
		}
		return nil
	}
	if len(result.Diagnostics) == 0 && len(remainingSkipped) == 0 {
		return nil
	}
	if _, err := fmt.Fprintln(writer, "\nDetector status:"); err != nil {
		return err
	}
	for _, diagnostic := range result.Diagnostics {
		message := strings.TrimSuffix(strings.TrimSpace(diagnostic.Message), ".")
		if _, err := fmt.Fprintf(writer, "- %s detector: %s - skipped where unavailable.\n", title(diagnostic.Detector), message); err != nil {
			return err
		}
	}
	for _, skipped := range remainingSkipped {
		if _, err := fmt.Fprintf(writer, "- Not fully compared: %s.\n", skipped); err != nil {
			return err
		}
	}
	return nil
}

func statusKey(value string) string {
	value = strings.ToLower(strings.ReplaceAll(value, "-", " "))
	value = strings.TrimPrefix(value, "listening ")
	value = strings.TrimPrefix(value, "some ")
	value = strings.TrimSpace(strings.TrimSuffix(value, " detector"))
	return value
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
	value = strings.ReplaceAll(value, "-", " ")
	runes := []rune(value)
	runes[0] = unicode.ToUpper(runes[0])
	return string(runes)
}

func sentence(value string) string {
	return strings.TrimSuffix(strings.TrimSpace(value), ".")
}

func truncate(value string, limit int) string {
	if len(value) <= limit {
		return value
	}
	return value[:limit-1] + "..."
}
