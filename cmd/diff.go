package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/fatih/color"
	"github.com/spf13/cobra"

	"github.com/fadhlidev/snapdock/internal/docker"
	"github.com/fadhlidev/snapdock/internal/snapshot"
	"github.com/fadhlidev/snapdock/pkg/types"
)

var diffCmd = &cobra.Command{
	Use:   "diff <snapshot1.sfx> <snapshot2.sfx>",
	Short: "Compare two snapshots and show differences",
	Args:  cobra.ExactArgs(2),
	RunE:  runDiff,
}

func init() {
	rootCmd.AddCommand(diffCmd)
}

func runDiff(cmd *cobra.Command, args []string) error {
	sfx1 := args[0]
	sfx2 := args[1]

	// Validate inputs
	for _, sfx := range []string{sfx1, sfx2} {
		if _, err := os.Stat(sfx); err != nil {
			return fmt.Errorf("snapshot file not found: %s", sfx)
		}
		if !strings.HasSuffix(sfx, ".sfx") {
			return fmt.Errorf("file must have .sfx extension: %s", sfx)
		}
	}

	// Extract both snapshots
	extracted1, err := snapshot.Extract(sfx1)
	if err != nil {
		return fmt.Errorf("failed to extract %s: %w", sfx1, err)
	}
	defer extracted1.Cleanup()

	extracted2, err := snapshot.Extract(sfx2)
	if err != nil {
		return fmt.Errorf("failed to extract %s: %w", sfx2, err)
	}
	defer extracted2.Cleanup()

	// Header
	fmt.Println()
	fmt.Printf("%s %s\n", color.HiBlackString("---"), filepath.Base(sfx1))
	fmt.Printf("%s %s\n", color.HiBlackString("+++"), filepath.Base(sfx2))
	fmt.Println()

	// Image comparison
	color.New(color.Bold).Println("  Image:")
	if extracted1.Container.Image != extracted2.Container.Image {
		fmt.Printf("    %s %s\n", color.RedString("-"), extracted1.Container.Image)
		fmt.Printf("    %s %s\n", color.GreenString("+"), extracted2.Container.Image)
	} else {
		fmt.Printf("    %s\n", color.CyanString(extracted1.Container.Image))
	}
	fmt.Println()

	// Environment variables comparison
	color.New(color.Bold).Println("  Environment Variables:")

	// Check for encryption
	enc1 := fileExists(filepath.Join(extracted1.TempDir, "env.json.enc"))
	enc2 := fileExists(filepath.Join(extracted2.TempDir, "env.json.enc"))

	if enc1 || enc2 {
		if enc1 && enc2 {
			fmt.Printf("    %s (both encrypted)\n", color.YellowString("Encrypted - cannot diff"))
		} else if enc1 {
			fmt.Printf("    %s (snapshot1 encrypted)\n", color.YellowString("Encrypted - cannot diff"))
		} else {
			fmt.Printf("    %s (snapshot2 encrypted)\n", color.YellowString("Encrypted - cannot diff"))
		}
	} else {
		env1 := parseEnvMap(extracted1.TempDir)
		env2 := parseEnvMap(extracted2.TempDir)
		diffEnvVars(env1, env2)
	}
	fmt.Println()

	// Port bindings comparison
	color.New(color.Bold).Println("  Port Bindings:")
	diffPorts(extracted1.Container.Ports, extracted2.Container.Ports)
	fmt.Println()

	// Mounts comparison
	color.New(color.Bold).Println("  Mounts:")
	diffMounts(extracted1.Container.Mounts, extracted2.Container.Mounts)

	return nil
}

func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

func parseEnvMap(tempDir string) map[string]string {
	envPath := filepath.Join(tempDir, "env.json")
	result := make(map[string]string)

	data, err := os.ReadFile(envPath)
	if err != nil {
		return result
	}

	var envVars []types.EnvVar
	if err := json.Unmarshal(data, &envVars); err != nil {
		return result
	}

	for _, e := range envVars {
		result[e.Key] = e.Value
	}
	return result
}

func diffEnvVars(env1, env2 map[string]string) {
	allKeys := make(map[string]bool)
	for k := range env1 {
		allKeys[k] = true
	}
	for k := range env2 {
		allKeys[k] = true
	}

	var keys []string
	for k := range allKeys {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	hasDiff := false
	for _, k := range keys {
		v1, ok1 := env1[k]
		v2, ok2 := env2[k]

		if !ok1 && ok2 {
			fmt.Printf("    %s %s=%s\n", color.GreenString("+"), k, maskValue(k, v2))
			hasDiff = true
		} else if ok1 && !ok2 {
			fmt.Printf("    %s %s=%s\n", color.RedString("-"), k, maskValue(k, v1))
			hasDiff = true
		} else if v1 != v2 {
			fmt.Printf("    %s %s=%s\n", color.RedString("-"), k, maskValue(k, v1))
			fmt.Printf("    %s %s=%s\n", color.GreenString("+"), k, maskValue(k, v2))
			hasDiff = true
		}
	}

	if !hasDiff {
		fmt.Printf("    %s\n", color.HiBlackString("(no differences)"))
	}
}

func maskValue(key, value string) string {
	lowerKey := strings.ToLower(key)
	sensitive := []string{"password", "secret", "token", "key", "api", "auth"}
	for _, s := range sensitive {
		if strings.Contains(lowerKey, s) {
			return "***"
		}
	}
	return value
}

func diffPorts(ports1, ports2 []docker.PortMapping) {
	makePortMap := func(ports []docker.PortMapping) map[string]string {
		m := make(map[string]string)
		for _, p := range ports {
			key := p.ContainerPort
			if p.HostPort != "" {
				m[key] = p.HostIP + ":" + p.HostPort
			}
		}
		return m
	}

	m1 := makePortMap(ports1)
	m2 := makePortMap(ports2)

	allPorts := make(map[string]bool)
	for k := range m1 {
		allPorts[k] = true
	}
	for k := range m2 {
		allPorts[k] = true
	}

	var ports []string
	for p := range allPorts {
		ports = append(ports, p)
	}
	sort.Strings(ports)

	hasDiff := false
	for _, p := range ports {
		v1, ok1 := m1[p]
		v2, ok2 := m2[p]

		if !ok1 && ok2 {
			fmt.Printf("    %s %s → %s\n", color.GreenString("+"), p, v2)
			hasDiff = true
		} else if ok1 && !ok2 {
			fmt.Printf("    %s %s → %s\n", color.RedString("-"), p, v1)
			hasDiff = true
		} else if v1 != v2 {
			fmt.Printf("    %s %s → %s\n", color.RedString("-"), p, v1)
			fmt.Printf("    %s %s → %s\n", color.GreenString("+"), p, v2)
			hasDiff = true
		}
	}

	if !hasDiff {
		fmt.Printf("    %s\n", color.HiBlackString("(no differences)"))
	}
}

func diffMounts(mounts1, mounts2 []docker.MountInfo) {
	type mountKey struct {
		dest string
		typ  string
	}

	makeMountMap := func(mounts []docker.MountInfo) map[mountKey]string {
		m := make(map[mountKey]string)
		for _, mnt := range mounts {
			key := mountKey{dest: mnt.Destination, typ: mnt.Type}
			var val string
			if mnt.Type == "volume" {
				val = mnt.Name
			} else {
				val = mnt.Source
			}
			m[key] = val
		}
		return m
	}

	m1 := makeMountMap(mounts1)
	m2 := makeMountMap(mounts2)

	allMounts := make(map[mountKey]bool)
	for k := range m1 {
		allMounts[k] = true
	}
	for k := range m2 {
		allMounts[k] = true
	}

	var keys []mountKey
	for k := range allMounts {
		keys = append(keys, k)
	}
	sort.Slice(keys, func(i, j int) bool {
		if keys[i].dest == keys[j].dest {
			return keys[i].typ < keys[j].typ
		}
		return keys[i].dest < keys[j].dest
	})

	hasDiff := false
	for _, k := range keys {
		v1, ok1 := m1[k]
		v2, ok2 := m2[k]

		if !ok1 && ok2 {
			fmt.Printf("    %s %s: %s → %s\n", color.GreenString("+"), k.typ, v2, k.dest)
			hasDiff = true
		} else if ok1 && !ok2 {
			fmt.Printf("    %s %s: %s → %s\n", color.RedString("-"), k.typ, v1, k.dest)
			hasDiff = true
		} else if v1 != v2 {
			fmt.Printf("    %s %s: %s → %s\n", color.RedString("-"), k.typ, v1, k.dest)
			fmt.Printf("    %s %s: %s → %s\n", color.GreenString("+"), k.typ, v2, k.dest)
			hasDiff = true
		}
	}

	if !hasDiff {
		fmt.Printf("    %s\n", color.HiBlackString("(no differences)"))
	}
}