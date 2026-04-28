package compose

import (
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

var DefaultComposeFiles = []string{
	"docker-compose.yml",
	"docker-compose.yaml",
	"compose.yml",
	"compose.yaml",
}

func FindComposeFile(dir string) (string, error) {
	for _, name := range DefaultComposeFiles {
		path := filepath.Join(dir, name)
		if info, err := os.Stat(path); err == nil && !info.IsDir() {
			return path, nil
		}
	}
	return "", fmt.Errorf("no compose file found in %s", dir)
}

type Project struct {
	Name       string
	FilePath  string
	Services  []Service
	Networks []Network
	Volumes  []Volume
}

type Service struct {
	Name     string
	Image   string
	Build   ServiceBuild
	Ports   []PortMapping
	Volumes []ServiceVolume
	EnvFile string
	Command string
}

type ServiceBuild struct {
	Context    string
	Dockerfile string
}

type PortMapping struct {
	Target    int
	Published int
	Protocol  string
}

type ServiceVolume struct {
	Type        string
	Source      string
	Target      string
	ReadOnly    bool
}

type Network struct {
	Name   string
	Driver string
}

type Volume struct {
	Name string
	Driver string
}

type composeDoc struct {
	Version  string `yaml:"version"`
	Services map[string]serviceConfig `yaml:"services"`
	Networks map[string]networkConfig `yaml:"networks"`
	Volumes  map[string]volumeDriverConfig `yaml:"volumes"`
	Name     string `yaml:"name"`
}

type serviceConfig struct {
	Image      string            `yaml:"image"`
	Build      buildConfig      `yaml:"build"`
	Command    string           `yaml:"command"`
	Ports      []portConfig     `yaml:"ports"`
	Volumes    []volumeBindConfig `yaml:"volumes"`
	EnvFile    string           `yaml:"env_file"`
	Environment map[string]string `yaml:"environment"`
}

type buildConfig struct {
	Context    string `yaml:"context"`
	Dockerfile string `yaml:"dockerfile"`
}

type portConfig struct {
	Target    any    `yaml:"target"`
	Published any   `yaml:"published"`
	Protocol  string `yaml:"protocol"`
}

type volumeBindConfig struct {
	Type     string `yaml:"type"`
	Source   string `yaml:"source"`
	Target   string `yaml:"target"`
	ReadOnly bool   `yaml:"read_only"`
}

type networkConfig struct {
	Driver string `yaml:"driver"`
}

type volumeDriverConfig struct {
	Driver string `yaml:"driver"`
}

func ParseComposeFile(path string) (*Project, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read compose file: %w", err)
	}

	var doc composeDoc
	if err := yaml.Unmarshal(data, &doc); err != nil {
		return nil, fmt.Errorf("parse compose file: %w", err)
	}

	project := &Project{
		FilePath: path,
		Name:     doc.Name,
		Services: make([]Service, 0, len(doc.Services)),
		Networks: make([]Network, 0, len(doc.Networks)),
		Volumes:  make([]Volume, 0, len(doc.Volumes)),
	}

	if project.Name == "" {
		project.Name = filepath.Base(filepath.Dir(path))
	}

	for name, cfg := range doc.Services {
		svc := Service{Name: name}
		if cfg.Image != "" {
			svc.Image = cfg.Image
		}
		if cfg.Build.Context != "" {
			svc.Build = ServiceBuild{
				Context:    cfg.Build.Context,
				Dockerfile: cfg.Build.Dockerfile,
			}
		}
		if cfg.Command != "" {
			svc.Command = cfg.Command
		}
		if cfg.EnvFile != "" {
			svc.EnvFile = cfg.EnvFile
		}
		for _, p := range cfg.Ports {
			svc.Ports = append(svc.Ports, PortMapping{
				Target:    toInt(p.Target),
				Published: toInt(p.Published),
				Protocol: p.Protocol,
			})
		}
		for _, v := range cfg.Volumes {
			svc.Volumes = append(svc.Volumes, ServiceVolume{
				Type:     v.Type,
				Source:   v.Source,
				Target:   v.Target,
				ReadOnly: v.ReadOnly,
			})
		}
		project.Services = append(project.Services, svc)
	}

	for name, cfg := range doc.Networks {
		net := Network{Name: name}
		if cfg.Driver != "" {
			net.Driver = cfg.Driver
		}
		project.Networks = append(project.Networks, net)
	}

	for name, cfg := range doc.Volumes {
		vol := Volume{Name: name}
		if cfg.Driver != "" {
			vol.Driver = cfg.Driver
		}
		project.Volumes = append(project.Volumes, vol)
	}

	return project, nil
}

func toInt(v any) int {
	if v == nil {
		return 0
	}
	switch n := v.(type) {
	case int:
		return n
	case float64:
		return int(n)
	case string:
		var i int
		fmt.Sscanf(n, "%d", &i)
		return i
	}
	return 0
}