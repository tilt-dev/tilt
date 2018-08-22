package demo

import (
	"fmt"
	"os"
	"time"

	"github.com/spf13/cobra"
)

func Execute() {
	rootCmd := &cobra.Command{
		Use: "tiltdemo",
	}

	if err := rootCmd.Execute(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}

	println(0, "Starting tilt...")
	println(100, "Building [my-app-frontend]")
	println(500, "Build comlete in 3.3423s")
}

func println(ms int, msg string) {
	time.Sleep(time.Duration(ms) * time.Millisecond)
	fmt.Println(msg)
}
