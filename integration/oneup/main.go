package main

import (
	"log"
	"net/http"
)

func main() {
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		msg := "🍄 One-Up! 🍄"
		_, _ = w.Write([]byte(msg))
	})

	log.Println("Serving oneup on 8000")
	_ = http.ListenAndServe(":8000", nil)
}
