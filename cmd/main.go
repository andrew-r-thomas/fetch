package main

import (
	"net/http"

	"github.com/andrew-r-thomas/fetch"
)

func main() {
	// fs := os.DirFS("/content")
	fs := fetch.Cache{}
	http.Handle("/", http.FileServerFS(&fs))
}
