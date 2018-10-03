package main

import (
	"net/http"
)

func main() {
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		msg := "🍄 One-Up! 🍄"
		_, _ = w.Write([]byte(msg))
	})

	_ = http.ListenAndServe(":8000", nil)
}
