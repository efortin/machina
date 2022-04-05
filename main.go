package main

import (
	"github.com/efortin/vz/pkg"
	"github.com/gin-gonic/gin"
	"net/http"
	"os"
	"os/exec"
)

func main() {

	argsWithoutProg := os.Args[1:]
	if len(argsWithoutProg) > 0 && argsWithoutProg[0] == "machine" {
		machineMode()
	} else {
		serverMode()
	}

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

func serverMode() {
	r := gin.Default()

	cmd := exec.Command("/Users/manu/Projects/vz/virtualization", "machine")
	err := cmd.Start()
	println(cmd.Process.Pid)
	println(err)
	cmd.Wait()

	r.GET("/virtual-machine/launch", func(c *gin.Context) {
		//go launch("test-1", "focal")
		c.JSON(http.StatusOK, gin.H{"data": "VM is launched"})
	})

	r.Run("127.0.0.1:8080")
}
