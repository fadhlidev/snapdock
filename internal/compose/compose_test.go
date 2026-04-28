package compose

import (
	"os"
	"path/filepath"
	"testing"
)

func TestFindComposeFile(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "snapdock-test-*")
	if err != nil {
		t.Fatalf("create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	found, err := FindComposeFile(tmpDir)
	if err == nil {
		t.Errorf("expected error when no compose file exists, got %q", found)
	}

	testFiles := []string{
		"docker-compose.yml",
		"docker-compose.yaml",
		"compose.yml",
		"compose.yaml",
	}

	for _, name := range testFiles {
		composePath := filepath.Join(tmpDir, name)
		if err := os.WriteFile(composePath, []byte("services: {}"), 0o644); err != nil {
			t.Fatalf("write file: %v", err)
		}

		found, err := FindComposeFile(tmpDir)
		if err != nil {
			t.Errorf("find compose file %q: %v", name, err)
		}
		if found != composePath {
			t.Errorf("expected %q, got %q", composePath, found)
		}

		os.Remove(composePath)
	}
}

func TestParseComposeFile(t *testing.T) {
	composeContent := `
version: "3.8"
name: myproject

services:
  web:
    image: nginx:latest
    ports:
      - target: 80
        published: "80"
      - target: 443
        published: "443"
    environment:
      NODE_ENV: production
    volumes:
      - type: bind
        source: ./data
        target: /usr/share/nginx/html

  db:
    image: postgres:15
    environment:
      POSTGRES_PASSWORD: secret
    volumes:
      - type: volume
        source: dbdata
        target: /var/lib/postgresql/data

networks:
  frontend:
    driver: bridge

volumes:
  dbdata:
    driver: local
`
	tmpDir, err := os.MkdirTemp("", "snapdock-test-*")
	if err != nil {
		t.Fatalf("create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	composePath := filepath.Join(tmpDir, "docker-compose.yml")
	if err := os.WriteFile(composePath, []byte(composeContent), 0o644); err != nil {
		t.Fatalf("write compose file: %v", err)
	}

	project, err := ParseComposeFile(composePath)
	if err != nil {
		t.Fatalf("parse compose file: %v", err)
	}

	if project.Name != "myproject" {
		t.Errorf("expected project name 'myproject', got %q", project.Name)
	}

	if len(project.Services) != 2 {
		t.Errorf("expected 2 services, got %d", len(project.Services))
	}

	webSvc := findService(project, "web")
	if webSvc == nil {
		t.Fatal("service 'web' not found")
	}
	if webSvc.Image != "nginx:latest" {
		t.Errorf("expected image 'nginx:latest', got %q", webSvc.Image)
	}
	if len(webSvc.Ports) != 2 {
		t.Errorf("expected 2 ports, got %d", len(webSvc.Ports))
	}

	dbSvc := findService(project, "db")
	if dbSvc == nil {
		t.Fatal("service 'db' not found")
	}
	if dbSvc.Image != "postgres:15" {
		t.Errorf("expected image 'postgres:15', got %q", dbSvc.Image)
	}

	if len(project.Networks) != 1 {
		t.Errorf("expected 1 network, got %d", len(project.Networks))
	}
	if project.Networks[0].Name != "frontend" {
		t.Errorf("expected network 'frontend', got %q", project.Networks[0].Name)
	}

	if len(project.Volumes) != 1 {
		t.Errorf("expected 1 volume, got %d", len(project.Volumes))
	}
	if project.Volumes[0].Name != "dbdata" {
		t.Errorf("expected volume 'dbdata', got %q", project.Volumes[0].Name)
	}
}

func TestParseComposeFile_DefaultName(t *testing.T) {
	composeContent := `services: {}`
	tmpDir, err := os.MkdirTemp("", "snapdock-test-*")
	if err != nil {
		t.Fatalf("create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	subDir := filepath.Join(tmpDir, "myproject")
	if err := os.MkdirAll(subDir, 0o755); err != nil {
		t.Fatalf("create sub dir: %v", err)
	}

	composePath := filepath.Join(subDir, "docker-compose.yml")
	if err := os.WriteFile(composePath, []byte(composeContent), 0o644); err != nil {
		t.Fatalf("write compose file: %v", err)
	}

	project, err := ParseComposeFile(composePath)
	if err != nil {
		t.Fatalf("parse compose file: %v", err)
	}

	if project.Name != "myproject" {
		t.Errorf("expected project name 'myproject', got %q", project.Name)
	}
}

func TestParseComposeFile_WithBuild(t *testing.T) {
	composeContent := `
services:
  app:
    build:
      context: ./app
      dockerfile: Dockerfile.prod
    command: npm start
`
	tmpDir, err := os.MkdirTemp("", "snapdock-test-*")
	if err != nil {
		t.Fatalf("create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	composePath := filepath.Join(tmpDir, "docker-compose.yml")
	if err := os.WriteFile(composePath, []byte(composeContent), 0o644); err != nil {
		t.Fatalf("write compose file: %v", err)
	}

	project, err := ParseComposeFile(composePath)
	if err != nil {
		t.Fatalf("parse compose file: %v", err)
	}

	appSvc := findService(project, "app")
	if appSvc == nil {
		t.Fatal("service 'app' not found")
	}
	if appSvc.Image != "" {
		t.Errorf("expected no image for build service, got %q", appSvc.Image)
	}
	if appSvc.Build.Context != "./app" {
		t.Errorf("expected build context './app', got %q", appSvc.Build.Context)
	}
	if appSvc.Build.Dockerfile != "Dockerfile.prod" {
		t.Errorf("expected dockerfile 'Dockerfile.prod', got %q", appSvc.Build.Dockerfile)
	}
	if appSvc.Command != "npm start" {
		t.Errorf("expected command 'npm start', got %q", appSvc.Command)
	}
}

func findService(p *Project, name string) *Service {
	for i := range p.Services {
		if p.Services[i].Name == name {
			return &p.Services[i]
		}
	}
	return nil
}