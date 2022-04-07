package utils

import (
	"github.com/withmandala/go-log"
	"os"
)

var (
	Logger = log.New(os.Stdout).WithDebug()
)
