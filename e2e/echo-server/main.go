package main

import (
	"fmt"
	"io"
	"net/http"
)

type echoHandler struct{}

func (echoHandler) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	body, err := io.ReadAll(req.Body)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	w.Header().Set("content-type", "application/octet-stream")
	if req.URL.Path == "/source" {
		w.Write([]byte(fmt.Sprintf("source: %s\n", req.RemoteAddr)))
	} else {
		w.Write(body)
	}
}

func main() {
	s := &http.Server{
		Handler: echoHandler{},
	}
	s.ListenAndServe()
}
