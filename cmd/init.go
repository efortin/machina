package cmd

import (
	internal "github.com/efortin/machina/pkg"
	"github.com/efortin/machina/utils"
	"github.com/spf13/cobra"
)

// Launch represents the Launch command
var InitCmd = &cobra.Command{
	Use:   "init",
	Short: "Start an existing machine using Apple Virtualization Framework",
	Long: `Start an existing machine using Apple Virtualization Framework.
You can use autocompletion ( read completion command)

start the default machine:
  machine start
same as:
  machine start primary

start the machine named ubuntu and override cpu with 2 cpu and 2 go of ram:
  machine start ubuntu --memory 2048 --cpu 3
`,
	Run: func(cmd *cobra.Command, args []string) {
		utils.Logger.Infof("initialize")
		internal.GenerateMachinaKeypair()
	},
}

func init() {
	RootCmd.AddCommand(InitCmd)
}
