package main

import (
	"net/http"

	"github.com/andrew-r-thomas/iwtbfs/fetch"
)

func main() {
	// fs := os.DirFS("/content")
	fs := fetch.CacheFS{}
	http.Handle("/", http.FileServerFS(&fs))
}
