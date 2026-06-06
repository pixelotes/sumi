package server

import (
	"fmt"
	"log"
	"net/http"

	"sumi/internal/config"

	"github.com/gorilla/mux"
)

type Server struct {
	cfg    *config.Config
	router *mux.Router
}

func New(cfg *config.Config) *Server {
	s := &Server{cfg: cfg, router: mux.NewRouter()}
	s.routes()
	return s
}

func (s *Server) Start() error {
	addr := fmt.Sprintf(":%d", s.cfg.App.Port)
	log.Printf("sumi listening on %s", addr)
	return http.ListenAndServe(addr, s.logRequests(s.router))
}

func (s *Server) routes() {
	// Static.
	s.router.PathPrefix("/js/").Handler(http.StripPrefix("/js/", http.FileServer(http.Dir("web/js"))))
	s.router.PathPrefix("/css/").Handler(http.StripPrefix("/css/", http.FileServer(http.Dir("web/css"))))
	s.router.PathPrefix("/vendor/").Handler(http.StripPrefix("/vendor/", http.FileServer(http.Dir("web/vendor"))))
	s.router.HandleFunc("/", s.serveIndex).Methods("GET")

	// Public auth.
	s.router.HandleFunc("/api/v1/login", s.handleLogin).Methods("POST")

	// Protected.
	api := s.router.PathPrefix("/api/v1").Subrouter()
	api.Use(s.jwtMiddleware)
	api.HandleFunc("/browse", s.handleBrowse).Methods("GET")
	api.HandleFunc("/image", s.handleImage).Methods("GET")
	api.HandleFunc("/cover", s.handleCover).Methods("GET")
	api.HandleFunc("/stream", s.handleStream).Methods("GET")
}

func (s *Server) serveIndex(w http.ResponseWriter, r *http.Request) {
	http.ServeFile(w, r, "web/index.html")
}

func (s *Server) logRequests(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		log.Printf("%s %s", r.Method, r.URL.Path)
		next.ServeHTTP(w, r)
	})
}
