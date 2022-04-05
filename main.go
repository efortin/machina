package main

import (
	"github.com/efortin/vz/pkg"
	"github.com/gin-gonic/gin"
	"net/http"
)

func main() {
	r := gin.Default()

	machine := internal.Machine{
		Name: "test-1",
		Distribution: &internal.UbuntuDistribution{
			ReleaseName:  "focal",
			Architecture: "arm64",
		},
	}
	machine.Launch()
	r.GET("/virtual-machine/launch", func(c *gin.Context) {
		//go launch("test-1", "focal")
		c.JSON(http.StatusOK, gin.H{"data": "VM is launched"})
	})

	r.Run("127.0.0.1:8080")
}
