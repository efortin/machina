/*
Copyright © 2022 NAME HERE <EMAIL ADDRESS>

*/
package node

import (
	"fmt"
	internal "github.com/efortin/machina/pkg"
	"github.com/efortin/machina/utils"
	"github.com/olekukonko/tablewriter"
	"github.com/spf13/cobra"
	"os"
	"strconv"
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
		t := tablewriter.NewWriter(os.Stdout)
		t.SetHeader([]string{"name", "status", "ip", "release", "aarch", "cpu", "memory", "process", "folder"})
		for _, mname := range vmlist.List() {
			machine, err := internal.FromFileSpec(mname)
			if err == nil {
				ip, _ := machine.IpAddress()
				t.Append([]string{
					machine.Name, machine.State(), ip, machine.Distribution.ReleaseName, machine.Distribution.Architecture, strconv.Itoa(int(machine.Spec.Cpu)), fmt.Sprint(machine.Spec.Ram/internal.GB, " GB"), machine.PID(), machine.BaseDirectory(),
				})
			} else {
				utils.NewSetFromSlice(mname, "error").List()
			}
		}
		t.Render()
	},
}

func init() {
	RootCmd.AddCommand(listCmd)
}
