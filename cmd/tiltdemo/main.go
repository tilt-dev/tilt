package main

import (
	"bufio"
	"fmt"
	"log"
	"os"
	"time"
)

func main() {
	printlnColor(0, "Starting tilt…")
	// printlnColor(20, "Found [GCR] context, using local docker daemon")
	printlnColor(100, "Building from scratch: [gcr.io/project/app-frontend]…")
	newline()
	println(100, "Sending build context to Docker daemon 20.23MB")
	println(100, " → tarring context")
	println(100, " → building image")
	newline()
	printDockerBuild()
	newline()
	println(100, "Parsing Kubernetes config YAML")
	println(100, " → applying via kubectl")
	println(100, "Successfully built 2917b4065035")
	println(100, "Successfully tagged 2917b4065035")
	printlnColor(200, "Build complete in 7.2342s")

	awaitInput()
	println(100, "Building [my-app-backend]")
	printlnColor(200, "Build complete in 1.2342s")

	awaitInput()
	println(100, "Building [my-app-backend2]")
	printlnColor(200, "Build complete in 2.1332s")
}

func newline() {
	fmt.Println("")
}

func println(ms int, msg string) {
	time.Sleep(time.Duration(ms) * time.Millisecond)
	fmt.Println(msg)
}

func printDockerBuild() {
	println(100, `   Step 1/6 : FROM iron/go:dev`)
	println(100, `     ---> bc624028dcfb`)
	println(200, `   Step 2/6 : ADD . /`)
	println(100, `     ---> Using cache`)
	println(100, `     ---> b8fe67c603de`)
	println(200, `   Step 3/6 : RUN ["sh", "-c", "cd /go/src/github.com/windmilleng/blorgly-backend; go get ./..."]`)
	println(100, `     ---> Using cache`)
	println(100, `     ---> da097bd0eea4`)
	println(200, `   Step 4/6 : RUN ["sh", "-c", "mkdir -p /app"]`)
	println(100, `     ---> Using cache`)
	println(100, `     ---> 5a14cca8a61c`)
	println(200, `   Step 5/6 : RUN ["sh", "-c", "cd /go/src/github.com/windmilleng/blorgly-backend; go build -o server; cp server /app/"]`)
	println(100, `     ---> Using cache`)
	println(100, `     ---> 0008e43da141`)
	println(200, `   Step 6/6 : ENTRYPOINT ["sh", "-c", "/app/server"]`)
	println(100, `     ---> Using cache`)
	println(100, `     ---> 2917b4065035`)
}

func printlnColor(ms int, msg string) {
	cGreen := "\033[32m"
	cReset := "\u001b[0m"
	time.Sleep(time.Duration(ms) * time.Millisecond)
	fmt.Printf("%s%s%s\n", cGreen, msg, cReset)
}

func awaitInput() {
	reader := bufio.NewReader(os.Stdin)
	_, err := reader.ReadByte()
	if err != nil {
		log.Fatal(err)
	}
}
