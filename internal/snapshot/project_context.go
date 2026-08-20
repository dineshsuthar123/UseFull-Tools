package snapshot

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
)

const maxContextFileSize = 1 << 20

var languageByExtension = map[string]string{
	".c": "C", ".cc": "C++", ".cpp": "C++", ".cs": "C#", ".go": "Go",
	".java": "Java", ".js": "JavaScript", ".jsx": "JavaScript", ".kt": "Kotlin",
	".kts": "Kotlin", ".php": "PHP", ".py": "Python", ".rb": "Ruby", ".rs": "Rust",
	".scala": "Scala", ".swift": "Swift", ".ts": "TypeScript", ".tsx": "TypeScript",
}

var buildSystemByFile = map[string]string{
	"pom.xml": "Maven", "build.gradle": "Gradle", "build.gradle.kts": "Gradle",
	"package.json": "Node/npm", "go.mod": "Go modules", "cargo.toml": "Cargo",
	"pyproject.toml": "Python/pyproject", "requirements.txt": "Python/pip",
}

var (
	environmentReferencePatterns = []*regexp.Regexp{
		regexp.MustCompile(`\$\{([A-Z][A-Z0-9_]{2,})`),
		regexp.MustCompile(`process\.env\.([A-Z][A-Z0-9_]{2,})`),
		regexp.MustCompile(`(?i)(?:getenv|env)\s*\(\s*["']([A-Z][A-Z0-9_]{2,})`),
		regexp.MustCompile(`System\.getenv\s*\(\s*["']([A-Z][A-Z0-9_]{2,})`),
		regexp.MustCompile(`\b([A-Z][A-Z0-9_]*(?:_HOST|_PORT|_URL))\b`),
	}
	configuredPortPattern   = regexp.MustCompile(`(?i)\bport\b\s*[:=]\s*["']?(\d{2,5})`)
	urlPortPattern          = regexp.MustCompile(`://[^\s"']+:([0-9]{2,5})`)
	pomArtifactPattern      = regexp.MustCompile(`(?s)<artifactId>\s*([^<\s]+)\s*</artifactId>`)
	gradleDependencyPattern = regexp.MustCompile(`(?m)(?:implementation|api|runtimeOnly|testImplementation)\s*[\("']+([^:"']+):([^:"']+)`)
	composeServicePattern   = regexp.MustCompile(`^\s{2}([A-Za-z0-9_.-]+):\s*(?:#.*)?$`)
	composeImagePattern     = regexp.MustCompile(`^\s+image:\s*["']?([^\s"']+)`)
	composePortPattern      = regexp.MustCompile(`([0-9]{2,5})\s*:\s*([0-9]{2,5})`)
)

func captureProjectContext(ctx context.Context, root string, files map[string]FileState) (ProjectContext, bool, []Diagnostic) {
	languages := map[string]struct{}{}
	buildSystems := map[string]struct{}{}
	dependencies := map[string]struct{}{}
	configFiles := make([]string, 0)
	envReferences := map[string]map[string]struct{}{}
	portReferences := map[int]map[string]string{}
	services := map[string]ServiceRef{}
	diagnostics := make([]Diagnostic, 0)
	complete := true

	paths := make([]string, 0, len(files))
	for path := range files {
		paths = append(paths, path)
	}
	sort.Strings(paths)
	for _, path := range paths {
		if err := ctx.Err(); err != nil {
			return ProjectContext{}, false, append(diagnostics, Diagnostic{Detector: "project-context", Severity: "warning", Message: err.Error()})
		}
		state := files[path]
		if language := languageByExtension[strings.ToLower(filepath.Ext(path))]; language != "" {
			languages[language] = struct{}{}
		}
		base := strings.ToLower(filepath.Base(path))
		if buildSystem := buildSystemByFile[base]; buildSystem != "" {
			buildSystems[buildSystem] = struct{}{}
		}
		if state.Kind != "config" && state.Kind != "dependency" {
			continue
		}
		if state.Kind == "config" {
			configFiles = append(configFiles, path)
		}
		if state.Size > maxContextFileSize {
			continue
		}
		content, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(path)))
		if err != nil {
			complete = false
			if len(diagnostics) < 10 {
				diagnostics = append(diagnostics, Diagnostic{Detector: "project-context", Severity: "warning", Message: fmt.Sprintf("read %s: %v", path, err)})
			}
			continue
		}
		text := string(content)
		collectEnvironmentReferences(text, path, envReferences)
		collectConfiguredPorts(text, path, portReferences)
		collectDependencies(base, content, dependencies)
		if isComposeFile(base) {
			collectComposeContext(text, path, services, portReferences)
		}
	}

	return ProjectContext{
		Languages: sortedKeys(languages), BuildSystems: sortedKeys(buildSystems),
		Dependencies: sortedKeys(dependencies), Services: sortedServices(services),
		ReferencedPorts: flattenPorts(portReferences), ReferencedEnvironment: flattenEnvironment(envReferences),
		ConfigurationFiles: configFiles,
	}, complete, diagnostics
}

func collectDependencies(base string, content []byte, result map[string]struct{}) {
	switch base {
	case "package.json":
		var document struct {
			Dependencies    map[string]json.RawMessage `json:"dependencies"`
			DevDependencies map[string]json.RawMessage `json:"devDependencies"`
		}
		if json.Unmarshal(content, &document) == nil {
			for name := range document.Dependencies {
				result[name] = struct{}{}
			}
			for name := range document.DevDependencies {
				result[name] = struct{}{}
			}
		}
	case "pom.xml":
		for _, match := range pomArtifactPattern.FindAllSubmatch(content, 300) {
			result[string(match[1])] = struct{}{}
		}
	case "build.gradle", "build.gradle.kts":
		for _, match := range gradleDependencyPattern.FindAllSubmatch(content, 300) {
			result[string(match[1])+":"+string(match[2])] = struct{}{}
		}
	case "go.mod":
		for _, line := range strings.Split(string(content), "\n") {
			fields := strings.Fields(line)
			moduleIndex, versionIndex := 0, 1
			if len(fields) >= 3 && fields[0] == "require" {
				moduleIndex, versionIndex = 1, 2
			}
			if len(fields) > versionIndex && (strings.Contains(fields[moduleIndex], ".") || strings.Contains(fields[moduleIndex], "/")) && strings.HasPrefix(fields[versionIndex], "v") {
				result[fields[moduleIndex]] = struct{}{}
			}
		}
	}
}

func collectEnvironmentReferences(text, source string, result map[string]map[string]struct{}) {
	for _, pattern := range environmentReferencePatterns {
		for _, match := range pattern.FindAllStringSubmatch(text, 500) {
			name := strings.ToUpper(match[1])
			if result[name] == nil {
				result[name] = map[string]struct{}{}
			}
			result[name][source] = struct{}{}
		}
	}
}

func collectConfiguredPorts(text, source string, result map[int]map[string]string) {
	for _, pattern := range []*regexp.Regexp{configuredPortPattern, urlPortPattern} {
		for _, match := range pattern.FindAllStringSubmatch(text, 100) {
			port, err := strconv.Atoi(match[1])
			if err == nil && port > 0 && port <= 65535 {
				if result[port] == nil {
					result[port] = map[string]string{}
				}
				result[port][source] = ""
			}
		}
	}
}

func collectComposeContext(text, source string, services map[string]ServiceRef, ports map[int]map[string]string) {
	currentService := ""
	inServices := false
	inPorts := false
	for _, line := range strings.Split(strings.ReplaceAll(text, "\r\n", "\n"), "\n") {
		trimmed := strings.TrimSpace(line)
		if trimmed == "services:" {
			inServices = true
			currentService = ""
			continue
		}
		if !inServices {
			continue
		}
		indent := len(line) - len(strings.TrimLeft(line, " \t"))
		if indent == 0 && trimmed != "" && !strings.HasPrefix(trimmed, "#") {
			inServices = false
			continue
		}
		if match := composeServicePattern.FindStringSubmatch(line); match != nil {
			currentService = match[1]
			services[currentService] = ServiceRef{Name: currentService, Type: inferServiceType(currentService), Source: source}
			inPorts = false
			continue
		}
		if currentService == "" {
			continue
		}
		if match := composeImagePattern.FindStringSubmatch(line); match != nil {
			service := services[currentService]
			service.Image = match[1]
			if inferred := inferServiceType(currentService + " " + match[1]); inferred != "" {
				service.Type = inferred
			}
			services[currentService] = service
		}
		if strings.HasPrefix(trimmed, "ports:") {
			inPorts = true
			continue
		}
		if inPorts {
			if !strings.HasPrefix(trimmed, "-") {
				inPorts = false
				continue
			}
			if match := composePortPattern.FindStringSubmatch(trimmed); match != nil {
				port, err := strconv.Atoi(match[1])
				if err == nil && port > 0 && port <= 65535 {
					if ports[port] == nil {
						ports[port] = map[string]string{}
					}
					ports[port][source] = currentService
				}
			}
		}
	}
}

func inferServiceType(value string) string {
	lower := strings.ToLower(value)
	services := []struct{ fragment, serviceType string }{
		{"postgres", "PostgreSQL"}, {"redis", "Redis"}, {"mariadb", "MariaDB"}, {"mysql", "MySQL"},
		{"mongo", "MongoDB"}, {"elastic", "Elasticsearch"}, {"kafka", "Kafka"}, {"rabbit", "RabbitMQ"},
	}
	for _, service := range services {
		if strings.Contains(lower, service.fragment) {
			return service.serviceType
		}
	}
	return ""
}

func isComposeFile(base string) bool {
	return base == "docker-compose.yml" || base == "docker-compose.yaml" || base == "compose.yml" || base == "compose.yaml"
}

func sortedKeys(values map[string]struct{}) []string {
	result := make([]string, 0, len(values))
	for value := range values {
		result = append(result, value)
	}
	sort.Strings(result)
	return result
}

func sortedServices(values map[string]ServiceRef) []ServiceRef {
	result := make([]ServiceRef, 0, len(values))
	for _, value := range values {
		result = append(result, value)
	}
	sort.Slice(result, func(i, j int) bool { return result[i].Name < result[j].Name })
	return result
}

func flattenPorts(values map[int]map[string]string) []PortRef {
	result := make([]PortRef, 0)
	for port, sources := range values {
		for source, service := range sources {
			result = append(result, PortRef{Port: port, Source: source, Service: service})
		}
	}
	sort.Slice(result, func(i, j int) bool {
		if result[i].Port != result[j].Port {
			return result[i].Port < result[j].Port
		}
		if result[i].Source != result[j].Source {
			return result[i].Source < result[j].Source
		}
		return result[i].Service < result[j].Service
	})
	return result
}

func flattenEnvironment(values map[string]map[string]struct{}) []EnvReference {
	result := make([]EnvReference, 0)
	for name, sources := range values {
		for source := range sources {
			result = append(result, EnvReference{Name: name, Source: source})
		}
	}
	sort.Slice(result, func(i, j int) bool {
		if result[i].Name != result[j].Name {
			return result[i].Name < result[j].Name
		}
		return result[i].Source < result[j].Source
	})
	return result
}
