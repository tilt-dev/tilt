package main

import (
	"fmt"
	"time"
)

func main() {
	println(0, "Starting tilt...")
	println(100, "Building [my-app-frontend]")
	println(500, "Build complete in 3.3423s")
}

func println(ms int, msg string) {
	time.Sleep(time.Duration(ms) * time.Millisecond)
	fmt.Println(msg)
}
