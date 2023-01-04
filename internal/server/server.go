// Package server provides a simple http server.
package server

import (
	"context"
	"net/http"

	"github.com/pico-cs/mqtt-gateway/internal/logger"
)

// A Server represents a http server.
type Server struct {
	lg             logger.Logger
	config         *Config
	addr           string
	*http.ServeMux // embedd (provides Handle and HandleFunc)
	svr            *http.Server
}

// New returns a new server instance.
func New(lg logger.Logger, config *Config) *Server {
	mux := &http.ServeMux{}
	addr := config.addr()
	return &Server{
		lg:       lg,
		config:   config,
		addr:     addr,
		ServeMux: mux,
		svr:      &http.Server{Addr: addr, Handler: mux},
	}
}

// Addr returns the server address.
func (s *Server) Addr() string { return s.addr }

// ListenAndServe starts the server listening to new connections.
func (s *Server) ListenAndServe() error {
	// start http server listen and serve
	s.lg.Printf("connect to http server %s", s.addr)
	go func() {
		if err := s.svr.ListenAndServe(); err != http.ErrServerClosed {
			s.lg.Fatalf("http server ListenAndServe: %s", err)
		}
	}()
	return nil
}

// Close closes the http server.
func (s *Server) Close() error {
	// shutdown http server
	s.lg.Println("shutdown http server...")
	if err := s.svr.Shutdown(context.Background()); err != nil {
		// Error from closing listeners, or context timeout:
		s.lg.Printf("http server Shutdown: %v", err)
	}
	s.lg.Printf("disconnected from http server %s", s.addr)
	return nil
}
