package snapshot

import (
	"context"
	"fmt"
	"net"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"time"
)

func capturePorts(ctx context.Context, root string) (map[string]PortState, bool, []Diagnostic) {
	var output string
	var err error
	switch runtime.GOOS {
	case "windows":
		output, err = commandOutput(ctx, 2*time.Second, root, "netstat", "-ano", "-p", "tcp")
	case "linux":
		output, err = commandOutput(ctx, 2*time.Second, root, "ss", "-H", "-ltn")
	case "darwin":
		output, err = commandOutput(ctx, 2*time.Second, root, "lsof", "-nP", "-iTCP", "-sTCP:LISTEN")
	default:
		return nil, false, []Diagnostic{{Detector: "ports", Severity: "info", Message: "unsupported operating system"}}
	}
	if err != nil {
		return nil, false, []Diagnostic{{Detector: "ports", Severity: "info", Message: fmt.Sprintf("listening ports unavailable: %v", err)}}
	}
	ports := parsePorts(output, runtime.GOOS)
	return ports, true, nil
}

func parsePorts(output, operatingSystem string) map[string]PortState {
	result := make(map[string]PortState)
	lines := strings.Split(strings.ReplaceAll(output, "\r\n", "\n"), "\n")
	sort.Strings(lines)
	for _, line := range lines {
		fields := strings.Fields(line)
		var endpoint, owner string
		switch operatingSystem {
		case "windows":
			if len(fields) < 4 || !strings.EqualFold(fields[0], "TCP") || !strings.EqualFold(fields[len(fields)-2], "LISTENING") {
				continue
			}
			endpoint = fields[1]
		case "linux":
			if len(fields) < 4 || !strings.EqualFold(fields[0], "LISTEN") {
				continue
			}
			endpoint = fields[3]
		case "darwin":
			if len(fields) < 2 || strings.EqualFold(fields[0], "COMMAND") {
				continue
			}
			endpointIndex := len(fields) - 1
			if fields[endpointIndex] == "(LISTEN)" && endpointIndex > 0 {
				endpointIndex--
			}
			endpoint = fields[endpointIndex]
			owner = fields[0]
		}
		address, port, ok := splitEndpoint(endpoint)
		if !ok {
			continue
		}
		key := "tcp:" + strconv.Itoa(port)
		candidate := PortState{Protocol: "tcp", Address: address, Port: port, Owner: owner}
		if existing, found := result[key]; !found || preferAddress(candidate.Address, existing.Address) {
			result[key] = candidate
		}
	}
	return result
}

func splitEndpoint(endpoint string) (string, int, bool) {
	endpoint = strings.TrimSuffix(endpoint, " (LISTEN)")
	if host, portText, err := net.SplitHostPort(endpoint); err == nil {
		port, err := strconv.Atoi(portText)
		return host, port, err == nil
	}
	separator := strings.LastIndex(endpoint, ":")
	if separator < 0 || separator == len(endpoint)-1 {
		return "", 0, false
	}
	port, err := strconv.Atoi(endpoint[separator+1:])
	if err != nil {
		return "", 0, false
	}
	address := strings.Trim(endpoint[:separator], "[]")
	return address, port, true
}

func preferAddress(candidate, existing string) bool {
	priority := func(address string) int {
		switch address {
		case "0.0.0.0", "*", "::":
			return 2
		default:
			return 1
		}
	}
	return priority(candidate) > priority(existing)
}
