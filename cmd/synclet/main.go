package main

import (
	"fmt"
	"os"

	"github.com/windmilleng/tilt/internal/cli"
)

func main() {
	err := new(cli.SyncletCmd).Register().Execute()
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}
