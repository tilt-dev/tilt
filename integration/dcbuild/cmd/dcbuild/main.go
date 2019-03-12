package main

import (
	"log"
	"net/http"
)

// One service deployed with docker-compose
func main() {
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		msg := "🍄 One-Up! 🍄"
		_, _ = w.Write([]byte(msg))
	})

	log.Println("Serving onedc on 8000")
	_ = http.ListenAndServe(":8000", nil)
}
