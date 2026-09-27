// Command cub-argo is `cub argo`, a cub CLI plugin that reads an Argo CD
// estate as it is and shows the fleet ConfigHub would govern:
//
//	cub argo plan ./my-argo-repo --stage-label rollout-phase --stages canary,secondary,primary
//
// It follows the shape of `cub sveltos` (confighub/sveltos-confighub): plan is
// offline and changes nothing.
package main

import (
	"fmt"
	"os"

	"github.com/confighub/sdk/core/plugin"

	"github.com/confighub/examples/cub-argo/cmd"
)

func main() {
	// cub runs this binary with the hook environment set when it installs or
	// upgrades the plugin; HandleHook writes cub-plugin.yaml, so the manifest
	// can never drift from the commands the binary implements.
	manifest := plugin.Manifest{
		Name:    "argo",
		Version: cmd.Version(),
		Commands: []plugin.Command{{
			Name:    "argo",
			Summary: "Read an Argo CD estate and plan it into ConfigHub, one variant per cluster",
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
