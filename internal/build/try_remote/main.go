package main

import (
	"fmt"
	"log"

	"github.com/windmilleng/tilt/internal/build"
)

func main() {
	err := build.TryIt()
	if err != nil {
		log.Fatal(err)
	}

	fmt.Println("You successfully Tried It™!")
}
