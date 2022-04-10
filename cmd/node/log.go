/*
Copyright © 2022 NAME HERE <EMAIL ADDRESS>

*/
package node

import (
	internal "github.com/efortin/machina/pkg"
	"github.com/spf13/cobra"
)

// logCmd represents the log command
var logCmd = &cobra.Command{
	Use:   "log",
	Short: "Display boot the machine boot log",
	Long:  `Display boot the machine boot log, the output doesn't not contains ssh command !'`,
	Run: func(cmd *cobra.Command, args []string) {
		log(args[0])
	},
	ValidArgs: internal.ListExistingMachines().List(),
	Args:      cobra.ExactValidArgs(1),
}

func init() {
	RootCmd.AddCommand(logCmd)
}

func log(machineName string) {
	internal.Machine{Name: machineName}.Log()
}
