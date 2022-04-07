package daemon

import (
	"fmt"
	internal "github.com/efortin/machina/pkg"
	"github.com/efortin/machina/utils"
	"github.com/spf13/cobra"
	"math"
	"strconv"
)

const (
	default_cpu_number = 2
	default_mem_mb     = 2 * GB
	GB                 = 1024 * 1024 * 1024
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
		fmt.Println("Launch called")
		cpus, err := strconv.Atoi(cmd.Flag("cpu").Value.String())
		if err != nil {
			cpus = default_cpu_number
		}

		ram, err := strconv.Atoi(cmd.Flag("memory").Value.String())
		if err != nil {
			ram = default_mem_mb
		}

		machine := internal.Machine{
			Name: cmd.Flag("name").Value.String(),
			Distribution: &internal.UbuntuDistribution{
				ReleaseName:  "focal",
				Architecture: "arm64",
			},
			Spec: internal.MachineSpec{
				Cpu: uint(math.Min(float64(cpus), 8.0)),
				Ram: uint64(math.Min(float64(ram)*GB, 16*GB)),
			},
		}
		utils.Logger.Info("Spec are:", "cpu", machine.Spec.Cpu, ": ram", machine.Spec.Ram)
		machine.LaunchPrimaryBoot()
		machine.Launch()

	},
}

func init() {
	RootCmd.AddCommand(LaunchCmd)
	LaunchCmd.Flags().StringP("name", "n", "primary", "Unique machine name")
	LaunchCmd.Flags().IntP("memory", "m", default_mem_mb, "Ram / Memory in GB")
	LaunchCmd.Flags().IntP("cpu", "c", default_cpu_number, "Cpu/core to allocate")

}
