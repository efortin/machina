/*
Copyright © 2022 NAME HERE <EMAIL ADDRESS>

*/
package node

import (
	internal "github.com/efortin/machina/pkg"
	"github.com/efortin/machina/utils"
	"github.com/spf13/cobra"
)

// stopCmd represents the stop command
var stopCmd = &cobra.Command{
	Use:   "stop",
	Short: "A brief description of your command",
	Long: `A longer description that spans multiple lines and likely contains examples
and usage of using your command. For example:

Cobra is a CLI library for Go that empowers applications.
This application is a tool to generate the needed files
to quickly create a Cobra application.`,
	Run: func(cmd *cobra.Command, args []string) {
		m, err := internal.FromFileSpec(args[0])
		utils.Logger.Debug(err)
		if m.State() == internal.Machine_state_running {
			m.Stop()
		} else {
			utils.Logger.Warn("Machine is not running, state:", m.State())
		}
	},
	ValidArgs: internal.ListExistingMachines().List(),
	Args:      cobra.ExactValidArgs(1),
}

func init() {
	RootCmd.AddCommand(stopCmd)

	// Here you will define your flags and configuration settings.

	// Cobra supports Persistent Flags which will work for this command
	// and all subcommands, e.g.:
	// stopCmd.PersistentFlags().String("foo", "", "A help for foo")

	// Cobra supports local flags which will only run when this command
	// is called directly, e.g.:
	// stopCmd.Flags().BoolP("toggle", "t", false, "Help message for toggle")
}
