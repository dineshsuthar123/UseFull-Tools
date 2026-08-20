package snapshot

import "testing"

func TestParseWindowsPorts(t *testing.T) {
	input := `
  Proto  Local Address          Foreign Address        State           PID
  TCP    0.0.0.0:8080           0.0.0.0:0              LISTENING       100
  TCP    [::]:8080              [::]:0                 LISTENING       100
  TCP    127.0.0.1:5432         0.0.0.0:0              LISTENING       200
  TCP    127.0.0.1:6000         127.0.0.1:6001         ESTABLISHED     300
`
	ports := parsePorts(input, "windows")
	if len(ports) != 2 {
		t.Fatalf("got %d ports, want 2: %#v", len(ports), ports)
	}
	if got := ports["tcp:8080"].Address; got != "0.0.0.0" {
		t.Fatalf("8080 address=%q, want wildcard IPv4", got)
	}
	if got := ports["tcp:5432"].Port; got != 5432 {
		t.Fatalf("port=%d, want 5432", got)
	}
}

func TestParseLinuxPorts(t *testing.T) {
	input := "LISTEN 0 4096 127.0.0.1:3000 0.0.0.0:*\nLISTEN 0 128 [::]:8080 [::]:*\n"
	ports := parsePorts(input, "linux")
	if len(ports) != 2 {
		t.Fatalf("got %d ports, want 2: %#v", len(ports), ports)
	}
}

func TestParseDarwinPorts(t *testing.T) {
	input := "COMMAND PID USER FD TYPE DEVICE SIZE/OFF NODE NAME\nnode 42 user 21u IPv4 0x0 0t0 TCP 127.0.0.1:5173 (LISTEN)\n"
	ports := parsePorts(input, "darwin")
	state, found := ports["tcp:5173"]
	if !found {
		t.Fatalf("port 5173 missing: %#v", ports)
	}
	if state.Owner != "node" {
		t.Fatalf("owner=%q, want node", state.Owner)
	}
}
