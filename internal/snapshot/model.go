package snapshot

import "time"

const SchemaVersion = 1

type Snapshot struct {
	SchemaVersion int                       `json:"schemaVersion"`
	Label         string                    `json:"label"`
	CapturedAt    time.Time                 `json:"capturedAt"`
	Root          string                    `json:"root"`
	Trigger       Trigger                   `json:"trigger"`
	Files         map[string]FileState      `json:"files"`
	Environment   map[string]EnvState       `json:"environment"`
	Runtimes      map[string]RuntimeState   `json:"runtimes"`
	Git           *GitState                 `json:"git,omitempty"`
	Ports         map[string]PortState      `json:"ports"`
	Containers    map[string]ContainerState `json:"containers"`
	Complete      map[string]bool           `json:"complete"`
	Stats         Stats                     `json:"stats"`
	Diagnostics   []Diagnostic              `json:"diagnostics,omitempty"`
}

type Trigger struct {
	Kind       string        `json:"kind"`
	Command    []string      `json:"command,omitempty"`
	Note       string        `json:"note,omitempty"`
	Duration   time.Duration `json:"duration,omitempty"`
	ExitCode   int           `json:"exitCode,omitempty"`
	FinishedAt time.Time     `json:"finishedAt,omitzero"`
}

type FileState struct {
	SHA256 string `json:"sha256"`
	Size   int64  `json:"size"`
	Kind   string `json:"kind"`
}

type EnvState struct {
	SHA256   string `json:"sha256"`
	Value    string `json:"value,omitempty"`
	Redacted bool   `json:"redacted,omitempty"`
}

type RuntimeState struct {
	Version string `json:"version"`
}

type GitState struct {
	Branch string `json:"branch"`
	Commit string `json:"commit"`
	Dirty  int    `json:"dirtyFiles"`
}

type PortState struct {
	Protocol string `json:"protocol"`
	Address  string `json:"address"`
	Port     int    `json:"port"`
	Owner    string `json:"owner,omitempty"`
}

type ContainerState struct {
	Image string `json:"image"`
	State string `json:"state"`
}

type Stats struct {
	FilesHashed       int `json:"filesHashed"`
	FilesSkippedLarge int `json:"filesSkippedLarge"`
	FilesSkipped      int `json:"filesSkipped"`
}

type Diagnostic struct {
	Detector string `json:"detector"`
	Severity string `json:"severity"`
	Message  string `json:"message"`
}
