package snapshot

import (
	"context"
	"fmt"

	"github.com/docker/docker/api/types/network"
	"github.com/fadhlidev/snapdock/internal/docker"
)

// NetworkDetail extends docker.NetworkInfo with subnet/driver metadata
// fetched from the daemon — not available in container inspect alone.
type NetworkDetail struct {
	docker.NetworkInfo
	Subnet  string `json:"subnet"`
	Gateway string `json:"gateway"`
	Driver  string `json:"driver"`
	Scope   string `json:"scope"` // "local" | "swarm" | "global"
}

// ResolveNetworks takes the NetworkInfo list from a ContainerSnapshot
// and enriches each entry with subnet/driver/gateway by calling
// docker network inspect on the daemon.
//
// Errors are non-fatal per network: if a network cannot be inspected
// (e.g. it was deleted between container inspect and now), the entry
// is kept with empty enrichment fields and a warning is recorded.
func ResolveNetworks(
	ctx context.Context,
	client *docker.Client,
	snap *docker.ContainerSnapshot,
) ([]NetworkDetail, []string, error) {
	details := make([]NetworkDetail, 0, len(snap.Networks))
	warnings := []string{}

	for _, ni := range snap.Networks {
		detail := NetworkDetail{NetworkInfo: ni}

		netInfo, err := client.Raw().NetworkInspect(ctx, ni.NetworkID, network.InspectOptions{})
		if err != nil {
			warnings = append(warnings,
				fmt.Sprintf("could not inspect network %q (%s): %v", ni.Name, ni.NetworkID[:12], err),
			)
			details = append(details, detail)
			continue
		}

		detail.Driver = netInfo.Driver
		detail.Scope = netInfo.Scope

		// Extract subnet + gateway from the first IPAM config
		if len(netInfo.IPAM.Config) > 0 {
			detail.Subnet  = netInfo.IPAM.Config[0].Subnet
			detail.Gateway = netInfo.IPAM.Config[0].Gateway
		}

		details = append(details, detail)
	}

	return details, warnings, nil
}
