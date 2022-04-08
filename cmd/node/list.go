/*
Copyright © 2022 NAME HERE <EMAIL ADDRESS>

*/
package node

import (
	"fmt"
	internal "github.com/efortin/machina/pkg"
	"github.com/efortin/machina/utils"
	"github.com/jedib0t/go-pretty/table"
	"github.com/spf13/cobra"
	"os"
)

// listCmd represents the list command
var listCmd = &cobra.Command{
	Use:   "list",
	Short: "A brief description of your command",
	Long: `A longer description that spans multiple lines and likely contains examples
and usage of using your command. For example:

Cobra is a CLI library for Go that empowers applications.
This application is a tool to generate the needed files
to quickly create a Cobra application.`,
	Run: func(cmd *cobra.Command, args []string) {

		vmlist := internal.ListExistingMachines()
		t := table.NewWriter()
		t.SetOutputMirror(os.Stdout)
		t.AppendHeader(table.Row{"name", "status", "ip", "release", "aarch", "cpu", "memory", "folder"})
		for _, mname := range vmlist.List() {
			machine, err := internal.FromFileSpec(mname)
			if err == nil {
				ip, _ := machine.IpAddress()
				t.AppendRow(table.Row{
					machine.Name, "created", ip, machine.Distribution.ReleaseName, machine.Distribution.Architecture, machine.Spec.Cpu, fmt.Sprint(machine.Spec.Ram/internal.GB, " GB"), machine.BaseDirectory(),
				})
			} else {
				t.AppendRow(table.Row{
					mname, "error",
				})
			}
		}
		t.AppendFooter(table.Row{utils.Empty, utils.Empty, utils.Empty, utils.Empty, utils.Empty, utils.Empty, fmt.Sprint("Total: ", len(vmlist.List()))})
		t.SetStyle(table.StyleColoredBlackOnGreenWhite)
		t.Render()
	},
}

func init() {
	RootCmd.AddCommand(listCmd)
}
