package main

import (
	"bufio"
	"fmt"
	"log"
	"os"
	"time"
)

func main() {
	println(0, "Starting tilt...")
	println(100, "Building [my-app-frontend]")
	println(500, "Build complete in 3.3423s")

	awaitInput()

	println(100, "Building [my-app-backend]")
	println(500, "Build complete in 0.1232s")

	awaitInput()

	println(100, "Building [my-app-backend2]")
	println(500, "Build complete in 1.3532s")
}

func println(ms int, msg string) {
	time.Sleep(time.Duration(ms) * time.Millisecond)
	fmt.Println(msg)
}

func awaitInput() {
	reader := bufio.NewReader(os.Stdin)
	_, err := reader.ReadByte()
	if err != nil {
		log.Fatal(err)
	}
}
