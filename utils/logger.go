package utils

import (
	log "github.com/withmandala/go-log"
	"os"
)

var (
	Logger = log.New(os.Stdout).WithoutDebug()
)
