package snapshot

import "time"

const SchemaVersion = 2

type Snapshot struct {
	SchemaVersion int                       `json:"schemaVersion"`
	Label         string                    `json:"label"`
	CapturedAt    time.Time                 `json:"capturedAt"`
	Root          string                    `json:"root"`
	CommandID     string                    `json:"commandId,omitempty"`
	Trigger       Trigger                   `json:"trigger"`
	Files         map[string]FileState      `json:"files"`
	Environment   map[string]EnvState       `json:"environment"`
	Runtimes      map[string]RuntimeState   `json:"runtimes"`
	Git           *GitState                 `json:"git,omitempty"`
	Ports         map[string]PortState      `json:"ports"`
	Containers    map[string]ContainerState `json:"containers"`
	Project       ProjectContext            `json:"projectContext"`
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
	SHA256          string `json:"sha256,omitempty"`
	Size            int64  `json:"size"`
	ModTimeUnixNano int64  `json:"mtimeUnixNano,omitempty"`
	Kind            string `json:"kind"`
	Tracked         bool   `json:"tracked"`
	Reason          string `json:"reason,omitempty"`
}

type EnvState struct {
	SHA256      string `json:"sha256"`
	Value       string `json:"value,omitempty"`
	Sensitivity string `json:"sensitivity"`
	Redacted    bool   `json:"redacted,omitempty"`
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

type ProjectContext struct {
	Languages             []string       `json:"languages,omitempty"`
	BuildSystems          []string       `json:"buildSystems,omitempty"`
	Dependencies          []string       `json:"dependencies,omitempty"`
	Services              []ServiceRef   `json:"services,omitempty"`
	ReferencedPorts       []PortRef      `json:"referencedPorts,omitempty"`
	ReferencedEnvironment []EnvReference `json:"referencedEnvironment,omitempty"`
	ConfigurationFiles    []string       `json:"configurationFiles,omitempty"`
}

type ServiceRef struct {
	Name   string `json:"name"`
	Type   string `json:"type,omitempty"`
	Image  string `json:"image,omitempty"`
	Source string `json:"source"`
}

type PortRef struct {
	Port    int    `json:"port"`
	Service string `json:"service,omitempty"`
	Source  string `json:"source"`
}

type EnvReference struct {
	Name   string `json:"name"`
	Source string `json:"source"`
}

type Stats struct {
	FilesHashed       int `json:"filesHashed"`
	FileHashesReused  int `json:"fileHashesReused"`
	FilesSkippedLarge int `json:"filesSkippedLarge"`
	FilesSkipped      int `json:"filesSkipped"`
}

type Diagnostic struct {
	Detector string `json:"detector"`
	Severity string `json:"severity"`
	Message  string `json:"message"`
}
