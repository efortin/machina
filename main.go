package main

import (
	"fmt"
	"github.com/efortin/vz/pkg"
	"github.com/efortin/vz/utils"
	"github.com/gin-gonic/gin"
	"net/http"
	"os"
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
	_, mac := machine.Launch()
	fmt.Println("////////////////////////////////////////////////////////The mac is ", mac)
}

func serverMode() {
	r := gin.Default()

	machine := internal.Machine{
		Name: "test-2",
		Distribution: &internal.UbuntuDistribution{
			ReleaseName:  "focal",
			Architecture: "arm64",
		},
	}
	machine.LaunchPrimaryBoot()
	_, _ = machine.Launch()
	ip, _ := machine.IpAddress()

	utils.Logger.Info("ip address is :", ip)

	r.GET("/virtual-machine/launch", func(c *gin.Context) {
		//go launch("test-1", "focal")
		c.JSON(http.StatusOK, gin.H{"data": "VM is launched"})
	})

	r.Run("127.0.0.1:8080")
}
