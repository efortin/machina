package daemon

import (
	internal "github.com/efortin/machina/pkg"
	"github.com/efortin/machina/utils"
	"github.com/spf13/cobra"
)

// Launch represents the Launch command
var LaunchCmd = &cobra.Command{
	Use:   "launch",
	Short: "Launch a machine usig Apple Virtualization Framework",
	Long: `Launch a machine using Apple Virtualization Framework.
For the moment, only Ubuntu 20.04 is supported but we'll try to add support
for Centos and debian soon.
For example:

Launch a default machine:
  machine launch

Launch a machine named ubuntu with 2 cpu and 2 go of ram:
  machine launch --name ubuntu --memory 2 --cpu 2
or with shorthand:
  machine launch -n ubuntu -m 2 -c 2
`,
	Run: func(cmd *cobra.Command, args []string) {
		mname := cmd.Flag("name").Value.String()
		machine, err := internal.FromFileSpec(mname)
		if err != nil {
			utils.Logger.Fatalf("Cannot start machine %s, the spec file wasn't found or it is not valid. error: %v", mname, err)
		}
		machine.Run()
	},
}

func init() {
	RootCmd.AddCommand(LaunchCmd)
	LaunchCmd.Flags().StringP("name", "n", "primary", "Unique machine name")
}
