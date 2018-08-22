package main

import (
	"bufio"
	"fmt"
	"os"
	"time"
)

func main() {
	println(0, "Starting tilt...")
	println(100, "Building [my-app-frontend]")
	println(500, "Build complete in 3.3423s")

	awaitInput('\n')

	println(100, "Building [my-app-backend]")
	println(500, "Build complete in 0.1232s")
}

func println(ms int, msg string) {
	time.Sleep(time.Duration(ms) * time.Millisecond)
	fmt.Println(msg)
}

func awaitInput(input byte) {
	reader := bufio.NewReader(os.Stdin)
	c, err := reader.ReadByte()
	if err != nil {
		fmt.Println(err)
		return
	}

	if c == input {
		return
	}
}
