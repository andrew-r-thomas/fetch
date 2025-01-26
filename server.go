package fetch

import (
	"fmt"
	"net/http"
)

type Server struct {
	port  int
	cache Cache
}

func NewServer() Server {
	s := Server{}
	http.HandleFunc("/", s.Handle)
	return s
}

func (s *Server) Handle(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodGet {
		r.URL.Path
	} else {
		// TODO:
	}
}
func (s *Server) Start() error {
	return http.ListenAndServe(fmt.Sprintf(":%d", s.port), nil)
}
