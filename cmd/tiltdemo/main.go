package main

import (
	"bufio"
	"fmt"
	"log"
	"os"
	"time"
)

// Colors
var cGreen = "\033[32m"
var cMagenta = "\033[35m"
var cBlue = "\033[34m"
var cReset = "\u001b[0m"

// Snippets
var linebreak = "\n"
var stream = "    │ "
var service = "app-frontend"

func main() {
	printlnColor(0, "Starting tilt…", cGreen)
	println(20, "  → current-context [docker-for-desktop]")

	firstPipeline()

	newline()
	newline()
	printlnColor(100, "Awaiting your edits…", cGreen)
	awaitInput()
	printlnColor(200, "  → File edited [app/main.go]", cBlue)

	secondPipeline()
}

func firstPipeline() {
	newline()
	printlnColor(100, "────┤ Pipeline 001 ├────────────────────────────────────────", cBlue)
	printlnColor(100, "STEP 1/3 — Building from scratch: [gcr.io/project/app-frontend]", cGreen)
	println(100, "  → tarring Docker context")
	println(100, "  → sending to Docker daemon…")
	printDockerOutput(3)
	printf(200, "    (Payload — %s5.230s%s)\n", cMagenta, cReset)

	newline()
	printf(100, "%sSTEP 2/3 — Pushing: [%s]%s\n", cGreen, service, cReset)
	println(200, "    (Done — 0.112s)")

	newline()
	printf(100, "%sSTEP 3/3 — Deploying: [%s]%s", cGreen, service, cReset)
	println(100, "  → parsing config YAML")
	println(100, "  → applying config via kubectl")
	printK8sOutput()
	println(200, "    (Done — 2.267s)")

	newline()
	printlnColor(100, "────┤ ︎Pipeline 001 Done ⚡︎├────────────────────────────────", cBlue)
	println(200, "    │ Overhead — 2.379s")
	printf(200, "    │  Payload — %s5.230s%s\n", cMagenta, cReset)
	printf(200, "    │    TOTAL — %s7.609s%s\n", cBlue, cReset)
}

func secondPipeline() {
	newline()
	printlnColor(100, "────┤ Pipeline 002 ├────────────────────────────────────────", cBlue)
	printlnColor(100, "STEP 1/2 — Building from existing: [gcr.io/project/app-frontend]", cGreen)
	println(100, "  → tarring Docker context")
	println(100, "  → sending to Docker daemon…")
	printDockerOutput(1)
	printf(200, "    (Payload — %s5.230s%s)\n", cMagenta, cReset)

	newline()
	printf(100, "%sSTEP 2/2 — Syncing: [%s]%s\n", cGreen, service, cReset)
	println(200, "    (Done — 0.232s)")

	newline()
	printlnColor(100, "────┤ ︎Pipeline 002 Done ⚡︎├────────────────────────────────", cBlue)
	println(200, "    │ Overhead — 0.232s")
	printf(200, "    │  Payload — %s5.230s%s\n", cMagenta, cReset)
	printf(200, "    │    TOTAL — %s5.642s%s\n", cBlue, cReset)
}

func printDockerOutput(factor int) {
	overhead := cMagenta + "    │ " + cReset

	newline()
	printf(100*factor, `%sStep 1/6 : FROM iron/go:dev%s`, stream, linebreak)
	printf(100*factor, `%s  ---> bc624028dcfb%s`, stream, linebreak)
	printf(100*factor, `%sStep 2/6 : ADD . /%s`, stream, linebreak)
	printf(100*factor, `%s  ---> Using cache%s`, stream, linebreak)
	printf(100*factor, `%s  ---> b8fe67c603de%s`, stream, linebreak)
	printf(200*factor, `%sStep 3/6 : RUN ["sh", "-c", "cd /go/src/github.com/windmilleng/app-frontend; go get ./..."]%s`, overhead, linebreak)
	printf(100*factor, `%s  ---> Using cache%s`, overhead, linebreak)
	printf(100*factor, `%s  ---> da097bd0eea4%s`, overhead, linebreak)
	printf(300*factor, `%sStep 4/6 : RUN ["sh", "-c", "mkdir -p /app"]%s`, overhead, linebreak)
	printf(100*factor, `%s  ---> Using cache%s`, overhead, linebreak)
	printf(100*factor, `%s  ---> 5a14cca8a61c%s`, overhead, linebreak)
	printf(200*factor, `%sStep 5/6 : RUN ["sh", "-c", "cd /go/src/github.com/windmilleng/app-frontend; go build -o server; cp server /app/"]%s`, overhead, linebreak)
	printf(100*factor, `%s  ---> Using cache%s`, overhead, linebreak)
	printf(100*factor, `%s  ---> 0008e43da141%s`, overhead, linebreak)
	printf(200*factor, `%sStep 6/6 : ENTRYPOINT ["sh", "-c", "/app/server"]%s`, stream, linebreak)
	printf(100*factor, `%s  ---> Using cache%s`, stream, linebreak)
	printf(100*factor, `%s  ---> 2917b4065035%s`, stream, linebreak)
	newline()
}

func printK8sOutput() {
	newline()
	printf(100, `%sservice "devel-hanyu-lb-blorgly-be" configured%s`, stream, linebreak)
	printf(100, `%sdeployment "devel-hanyu-blorgly-be" configured%s`, stream, linebreak)
	newline()
}

func newline() {
	fmt.Println("")
}

func println(ms int, msg string) {
	time.Sleep(time.Duration(ms) * time.Millisecond)
	fmt.Println(msg)
}

func printlnColor(ms int, msg string, color string) {
	time.Sleep(time.Duration(ms) * time.Millisecond)
	fmt.Printf("%s%s%s\n", color, msg, cReset)
}

func printf(ms int, format string, args ...interface{}) {
	time.Sleep(time.Duration(ms) * time.Millisecond)
	fmt.Printf(format, args...)
}

func awaitInput() {
	reader := bufio.NewReader(os.Stdin)
	_, err := reader.ReadByte()
	if err != nil {
		log.Fatal(err)
	}
}
