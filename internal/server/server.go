package server

import (
    "fmt"
    "net/http"

    "github.com/codyw912/ollama-gateway/internal/proxy"
)

type Server struct {
    Proxy *proxy.Proxy
    Port  string
}

func NewServer(p *proxy.Proxy, port string) *Server {
    return &Server{Proxy: p, Port: port}
}

func (s *Server) Start() error {
    http.HandleFunc("/", s.Proxy.Handler)
    fmt.Printf("Gateway running on port %s\n", s.Port)
    return http.ListenAndServe(":"+s.Port, nil)
}

