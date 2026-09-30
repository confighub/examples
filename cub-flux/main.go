// Command cub-flux is `cub flux`, a cub CLI plugin that reads a Flux fleet
// repository as it is and shows the fleet ConfigHub would govern:
//
//	cub flux plan ./my-fleet-repo --stages dev,staging,prod
//
// It follows the shape of `cub sveltos` (confighub/sveltos-confighub): plan is
// offline and changes nothing.
package main

import (
	"fmt"
	"os"

	"github.com/confighub/sdk/core/plugin"

	"github.com/confighub/examples/cub-flux/cmd"
)

func main() {
	// cub runs this binary with the hook environment set when it installs or
	// upgrades the plugin; HandleHook writes cub-plugin.yaml, so the manifest
	// can never drift from the commands the binary implements.
	manifest := plugin.Manifest{
		Name:    "flux",
		Version: cmd.Version(),
		Commands: []plugin.Command{{
			Name:    "flux",
			Summary: "Read a Flux fleet repository and plan it into ConfigHub, one variant per cluster",
		}},
	}
	if handled, err := plugin.HandleHook(manifest); handled {
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		return
	}
	cmd.Execute()
}
