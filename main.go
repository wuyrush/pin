package main

import (
	"io"
	"log"
	"net/http"
)

func main() {
	mainHandler := func(w http.ResponseWriter, req *http.Request) {
		io.WriteString(w, "welcome to pin!")
	}
	http.HandleFunc("/", mainHandler)
	log.Fatal(http.ListenAndServe(":8000", nil))
}
