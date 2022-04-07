package node

import (
	"fmt"
	internal "github.com/efortin/machina/pkg"
	"github.com/efortin/machina/utils"
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
		fmt.Println("Launch called")
		machine := internal.Machine{
			Name: cmd.Flag("name").Value.String(),
			Distribution: &internal.UbuntuDistribution{
				ReleaseName:  "focal",
				Architecture: "arm64",
			},
		}
		machine.Distribution.DownloadDistro()
		machine.BaseDirectory()

		ou, _ := os.Create(machine.BaseDirectory() + "/process.log")
		cwd, _ := os.Getwd()

		//args := append(os.Args, "--detached")
		mcmd := exec.Command("machina",
			"daemon", "launch",
			"-n", cmd.Flag("name").Value.String(),
			"-c", cmd.Flag("cpu").Value.String(),
			"-m", cmd.Flag("memory").Value.String(),
		)
		mcmd.Stderr = ou
		mcmd.Stdin = nil
		mcmd.Stdout = ou
		mcmd.Dir = cwd
		_ = mcmd.Start()
		pid := mcmd.Process.Pid
		utils.Logger.Info("the current process a pid", pid, mcmd.Args)
		mcmd.Process.Release()
		_ = os.WriteFile(machine.PidFilePath(), []byte(strconv.Itoa(pid)), 0600)

	},
}

func init() {
	RootCmd.AddCommand(LaunchCmd)
	LaunchCmd.Flags().StringP("name", "n", "primary", "Unique machine name")
	LaunchCmd.Flags().IntP("memory", "m", 2048, "Ram / Memory in MB")
	LaunchCmd.Flags().IntP("cpu", "c", 2, "Cpu/core to allocate")

}
