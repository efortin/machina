package main

import (
	"github.com/efortin/vz/pkg"
	"github.com/efortin/vz/utils"
	"os"
	"os/exec"
	"strconv"
)

func main() {

	argsWithoutProg := os.Args[1:]
	if len(argsWithoutProg) > 0 && argsWithoutProg[0] == "machine" {
		machineMode()
	} else if len(argsWithoutProg) > 0 && argsWithoutProg[0] == "control" {
		machineControl()
	} else {
		serverMode()
	}

}

func machineControl() {
	machine := internal.Machine{
		Name: "test-2",
		Distribution: &internal.UbuntuDistribution{
			ReleaseName:  "focal",
			Architecture: "arm64",
		},
	}
	machine.Kill()
}

func machineMode() {

	machine := internal.Machine{
		Name: "test-2",
		Distribution: &internal.UbuntuDistribution{
			ReleaseName:  "focal",
			Architecture: "arm64",
		},
	}
	machine.LaunchPrimaryBoot()
	machine.Launch()

}

const (
	UID  = 501
	GUID = 20
)

func serverMode() {

	machine := internal.Machine{
		Name: "test-2",
		Distribution: &internal.UbuntuDistribution{
			ReleaseName:  "focal",
			Architecture: "arm64",
		},
	}
	machine.Distribution.DownloadDistro()
	machine.BaseDirectory()

	ou, _ := os.Create(machine.BaseDirectory() + "/process.log")
	er, _ := os.Create(machine.BaseDirectory() + "/process.log")

	cwd, _ := os.Getwd()

	//args := append(os.Args, "--detached")
	cmd := exec.Command(os.Args[0], "machine")
	cmd.Stderr = er
	cmd.Stdin = nil
	cmd.Stdout = ou
	cmd.Dir = cwd
	_ = cmd.Start()
	pid := cmd.Process.Pid
	utils.Logger.Info("the current process a pid", pid)
	cmd.Process.Release()
	_ = os.WriteFile(machine.PidFilePath(), []byte(strconv.Itoa(pid)), 0600)

	//utils.Logger.Info("The machine", machine.Name, "has been successully created with the folowing ip:", ip)

}
