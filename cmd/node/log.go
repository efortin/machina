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
	Short: "A brief description of your command",
	Long: `A longer description that spans multiple lines and likely contains examples
and usage of using your command. For example:

Cobra is a CLI library for Go that empowers applications.
This application is a tool to generate the needed files
to quickly create a Cobra application.`,
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println("log called")
		fmt.Printf("%v", args)
		machine := internal.Machine{Name: args[0]}
		t, _ := tail.TailFile(machine.OutputFilePath(), tail.Config{Follow: true})
		for line := range t.Lines {
			fmt.Println(line.Text)
		}
	},
	ValidArgs: internal.ListExistingMachines(),
	Args:      cobra.ExactValidArgs(1),
}

func init() {
	RootCmd.AddCommand(logCmd)

}
