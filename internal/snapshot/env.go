package snapshot

import (
	"strings"

	"github.com/fadhlidev/snapdock/internal/docker"
	"github.com/fadhlidev/snapdock/pkg/types"
)

// ExtractEnv parses the raw KEY=VALUE env slice from a ContainerSnapshot
// into a structured []EnvVar slice.
//
// Docker stores env as ["KEY=VALUE", "KEY2=VALUE2", ...].
// Values themselves can contain "=" so we split on the first "=" only.
func ExtractEnv(snap *docker.ContainerSnapshot) []types.EnvVar {
	vars := make([]types.EnvVar, 0, len(snap.Env))

	for _, raw := range snap.Env {
		idx := strings.IndexByte(raw, '=')
		if idx < 0 {
			// Malformed entry — store as key with empty value
			vars = append(vars, types.EnvVar{Key: raw, Value: ""})
			continue
		}

		vars = append(vars, types.EnvVar{
			Key:   raw[:idx],
			Value: raw[idx+1:], // everything after the first "="
		})
	}

	return vars
}

// EnvToRaw converts []EnvVar back into Docker's KEY=VALUE format.
// Used during restore to reconstruct docker run --env flags.
func EnvToRaw(vars []types.EnvVar) []string {
	raw := make([]string, len(vars))
	for i, v := range vars {
		raw[i] = v.Key + "=" + v.Value
	}
	return raw
}
