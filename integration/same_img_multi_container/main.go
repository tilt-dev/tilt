package main

import (
	"fmt"
	"log"
	"net/http"
)

import "flag"

var port = flag.Int("port", 9999, "port to run server on")

func main() {
	flag.Parse()
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		msg := "🍄 One-Up! 🍄"
		log.Printf("Got HTTP request for %s", r.URL.Path)
		_, _ = w.Write([]byte(msg))
	})

	log.Printf("Serving oneup on container port %d\n", *port)
	err := http.ListenAndServe(fmt.Sprintf(":%d", *port), nil)
	if err != nil {
		log.Printf("SERVER DIED WITH ERROR:\n\t%v", err)
	}
}
