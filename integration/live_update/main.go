package main

import (
	"log"
	"net/http"
)

func main() {
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		msg := "🍄 One-Up! 🍄"
		log.Printf("Got HTTP request for %s", r.URL.Path)
		_, _ = w.Write([]byte(msg))
	})

	log.Println("Serving oneup on container port 8000")
	_ = http.ListenAndServe(":8000", nil)
}
