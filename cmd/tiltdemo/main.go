package main

import (
	"bufio"
	"fmt"
	"log"
	"os"
	"time"
)

var cGreen = "\033[32m"
var cBlue = "\033[34m"
var cReset = "\u001b[0m"

func main() {
	printlnColor(0, "Starting tilt!", cGreen)
	println(20, "  → in context [docker-for-desktop]")

	newline()
	printlnColor(100, "Building from scratch: [gcr.io/project/app-frontend]", cGreen)
	println(100, "  → tarring context")
	println(100, "  → sending to Docker daemon…")

	newline()
	printDockerBuild(3)

	newline()
	println(100, "  → tagged 2917b4065035")
	printlnColor(200, "  → Done in 7.234s", cBlue)

	newline()
	printlnColor(100, "Deploying: [devel-hanyu-lb-blorg-be]", cGreen)
	println(100, "  → parsing config YAML")
	println(100, "  → applying config via kubectl")
	println(100, "  → service created")
	println(100, "  → deployment created")
	printlnColor(200, "  → Done in 472ms", cBlue)

	newline()
	printlnColor(100, "Awaiting your edits…", cGreen)
	awaitInput()
	printlnColor(200, "  → File edited: main.go:123", cBlue)

	newline()
	printlnColor(100, "Building from existing: [gcr.io/project/app-frontend]", cGreen)
	println(100, "  → tarring context")
	println(100, "  → sending to Docker daemon…")

	newline()
	printDockerBuild(1)

	newline()
	println(100, "  → tagged 2917b4065035")
	printlnColor(200, "  → Done in 1.234s", cBlue)

	newline()
	printlnColor(100, "Deploying: [devel-hanyu-lb-blorg-be]", cGreen)
	println(100, "  → parsing config YAML")
	println(100, "  → applying config via kubectl")
	println(100, "  → service created")
	println(100, "  → deployment created")
	printlnColor(200, "  → Done in 472ms", cBlue)
	newline()

}

func newline() {
	fmt.Println("")
}

func println(ms int, msg string) {
	time.Sleep(time.Duration(ms) * time.Millisecond)
	fmt.Println(msg)
}

func printf(ms int, format string, args ...interface{}) {
	time.Sleep(time.Duration(ms) * time.Millisecond)
	fmt.Printf(format, args...)
}

func printDockerBuild(factor int) {
	println(100*factor, `    │ Step 1/6 : FROM iron/go:dev`)
	println(100*factor, `    │   ---> bc624028dcfb`)
	println(200*factor, `    │ Step 2/6 : ADD . /`)
	println(100*factor, `    │   ---> Using cache`)
	println(100*factor, `    │   ---> b8fe67c603de`)
	println(200*factor, `    │ Step 3/6 : RUN ["sh", "-c", "cd /go/src/github.com/windmilleng/blorgly-backend; go get ./..."]`)
	println(100*factor, `    │   ---> Using cache`)
	println(100*factor, `    │   ---> da097bd0eea4`)
	println(200*factor, `    │ Step 4/6 : RUN ["sh", "-c", "mkdir -p /app"]`)
	println(100*factor, `    │   ---> Using cache`)
	println(100*factor, `    │   ---> 5a14cca8a61c`)
	println(200*factor, `    │ Step 5/6 : RUN ["sh", "-c", "cd /go/src/github.com/windmilleng/blorgly-backend; go build -o server; cp server /app/"]`)
	println(100*factor, `    │   ---> Using cache`)
	println(100*factor, `    │   ---> 0008e43da141`)
	println(200*factor, `    │ Step 6/6 : ENTRYPOINT ["sh", "-c", "/app/server"]`)
	println(100*factor, `    │   ---> Using cache`)
	println(100*factor, `    │   ---> 2917b4065035`)
}

func printlnColor(ms int, msg string, color string) {
	time.Sleep(time.Duration(ms) * time.Millisecond)
	fmt.Printf("%s%s%s\n", color, msg, cReset)
}

func awaitInput() {
	reader := bufio.NewReader(os.Stdin)
	_, err := reader.ReadByte()
	if err != nil {
		log.Fatal(err)
	}
}
