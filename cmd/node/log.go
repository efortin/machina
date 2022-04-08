/*
Copyright © 2022 NAME HERE <EMAIL ADDRESS>

*/
package node

import (
	"fmt"
	internal "github.com/efortin/machina/pkg"
	"github.com/hpcloud/tail"
	"github.com/spf13/cobra"
)

// logCmd represents the log command
var logCmd = &cobra.Command{
	Use:   "log",
	Short: "Display boot the machine boot log",
	Long:  `Display boot the machine boot log, the output doesn't not contains ssh command !'`,
	Run: func(cmd *cobra.Command, args []string) {
		machine := internal.Machine{Name: args[0]}
		t, _ := tail.TailFile(machine.OutputFilePath(), tail.Config{Follow: true})
		for line := range t.Lines {
			fmt.Println(line.Text)
		}
	},
	ValidArgs: internal.ListExistingMachines().List(),
	Args:      cobra.ExactValidArgs(1),
}

func init() {
	RootCmd.AddCommand(logCmd)

}
