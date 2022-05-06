package webserver

import (
	"fmt"
	"net/http"
	"time"

	"github.com/gorilla/mux"
)

type Server struct {
	r   *mux.Router
	srv *http.Server
}

func New(port int) *Server {
	r := mux.NewRouter()
	return &Server{
		r: r,
		srv: &http.Server{
			Handler: r,
			Addr:    fmt.Sprintf(":%d", port),

			WriteTimeout: 15 * time.Second,
			ReadTimeout:  15 * time.Second,
		},
	}
}

func (s *Server) Run() error {
	return s.srv.ListenAndServe()
}

func (s *Server) AddHandler(path string, handler func(w http.ResponseWriter, r *http.Request)) {
	s.r.HandleFunc(path, handler)
}

func (s *Server) AddHandle(path string, handler http.Handler) {
	s.r.Handle(path, handler)
}
