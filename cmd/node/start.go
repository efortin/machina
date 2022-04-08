package node

import (
	internal "github.com/efortin/machina/pkg"
	"github.com/efortin/machina/utils"
	"os"
	"os/exec"
	"strconv"

	"github.com/spf13/cobra"
)

// Launch represents the Launch command
var StartCmd = &cobra.Command{
	Use:   "start",
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
	ValidArgs: internal.ListExistingMachines().List(),
	Args:      cobra.ExactValidArgs(1),
	Run: func(cmd *cobra.Command, args []string) {

		machineName := args[0]
		machine, error := internal.FromFileSpec(machineName)
		if error != nil {
			utils.Logger.Errorf("the configure machine %s can't be loaded, please fix it manually or delete it", machineName)
			os.Exit(1)
		}

		machine.Distribution.DownloadDistro()
		machine.BaseDirectory()

		ou, _ := os.Create(machine.BaseDirectory() + "/process.log")
		er, _ := os.Create(machine.BaseDirectory() + "/process_error.log")
		cwd, _ := os.Getwd()

		//args := append(os.Args, "--detached")
		mcmd := exec.Command(os.Args[0], "daemon", "launch", "-n", machineName)
		mcmd.Stderr = er
		mcmd.Stdin = nil
		mcmd.Stdout = ou
		mcmd.Dir = cwd
		_ = mcmd.Start()
		pid := mcmd.Process.Pid
		utils.Logger.Debug("the current process a pid: ", pid)
		mcmd.Process.Release()
		_ = os.WriteFile(machine.PidFilePath(), []byte(strconv.Itoa(pid)), 0600)

	},
}

func init() {
	RootCmd.AddCommand(StartCmd)
}
