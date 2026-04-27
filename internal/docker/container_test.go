package docker

import (
	"testing"

	"github.com/docker/go-connections/nat"
)

func TestBuildTmpfs(t *testing.T) {
	tmpfs := []string{"/tmp", "/var/run"}
	result := buildTmpfs(tmpfs)

	if len(result) != 2 {
		t.Errorf("expected 2 tmpfs entries, got %d", len(result))
	}
	if _, ok := result["/tmp"]; !ok {
		t.Errorf("/tmp missing from result")
	}
}

func TestBuildPortBindings(t *testing.T) {
	portBindings := map[string][]PortBinding{
		"80/tcp": {
			{HostIP: "0.0.0.0", HostPort: "8080"},
		},
	}

	result := buildPortBindings(portBindings)

	if len(result) != 1 {
		t.Errorf("expected 1 port mapping, got %d", len(result))
	}
	
	p80 := nat.Port("80/tcp")
	if len(result[p80]) != 1 {
		t.Errorf("expected 1 host binding for port 80, got %d", len(result[p80]))
	}
	if result[p80][0].HostPort != "8080" {
		t.Errorf("expected host port 8080, got %s", result[p80][0].HostPort)
	}
}
