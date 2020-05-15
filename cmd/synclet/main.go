package main

import (
	"fmt"
	"os"

	"github.com/tilt-dev/tilt/internal/cli"
)

func main() {
	err := new(cli.SyncletCmd).Register().Execute()
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}
