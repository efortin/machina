package node

import (
	internal "github.com/efortin/machina/pkg"
	"github.com/efortin/machina/utils"
	"math"
	"os"
	"os/exec"
	"strconv"

	"github.com/spf13/cobra"
)

// Launch represents the Launch command
var LaunchCmd = &cobra.Command{
	Use:   "launch",
	Short: "Launch a machine using Apple Virtualization Framework",
	Long: `Launch a machine using Apple Virtualization Framework.
For the moment, only Ubuntu 20.04 is supported but we'll try to add support
for Centos and debian soon.
For example:

Launch a default machine:
  machine Launch

Launch a machine named ubuntu with 2 cpu and 2 go of ram:
  machine Launch --name ubuntu --memory
`,
	Run: func(cmd *cobra.Command, args []string) {
		machineName := cmd.Flag("name").Value.String()
		if internal.ListExistingMachines().Contains(machineName) {
			utils.Logger.Errorf("The machine %s already exist, please use `start` command...", machineName)
			os.Exit(1)
		}
		cpus, err := strconv.Atoi(cmd.Flag("cpu").Value.String())
		if err != nil {
			cpus = internal.Default_cpu_number
		}

		ram, err := strconv.Atoi(cmd.Flag("memory").Value.String())
		if err != nil {
			ram = internal.Default_mem_mb
		}

		machine := &internal.Machine{
			Name: cmd.Flag("name").Value.String(),
			Distribution: &internal.UbuntuDistribution{
				ReleaseName:  "jammy",
				Architecture: "arm64",
			},
			Spec: internal.MachineSpec{
				Cpu: uint(math.Min(float64(cpus), 8.0)),
				Ram: uint64(math.Min(float64(ram)*internal.GB, 16*internal.GB)),
			},
		}

		machine.Distribution.DownloadDistro()
		machine.BaseDirectory()
		machine.RootDirectory()
		machine.ExportMachineSpecification()

		ou, _ := os.Create(machine.BaseDirectory() + "/process.log")
		cwd, _ := os.Getwd()

		//args := append(os.Args, "--detached")
		mcmd := exec.Command(os.Args[0], "daemon", "launch", "-n", machineName)
		mcmd.Stderr = ou
		mcmd.Stdin = nil
		mcmd.Stdout = ou
		mcmd.Dir = cwd
		_ = mcmd.Start()
		mcmd.Process.Release()

	},
}

func init() {
	RootCmd.AddCommand(LaunchCmd)
	LaunchCmd.Flags().StringP("name", "n", "primary", "Unique machine name")
	LaunchCmd.Flags().IntP("memory", "m", 2048, "Ram / Memory in MB")
	LaunchCmd.Flags().IntP("cpu", "c", 2, "Cpu/core to allocate")

}
