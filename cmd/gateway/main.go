package main

import (
    "log"

    "github.com/codyw912/ollama-gateway/internal/config"
    "github.com/codyw912/ollama-gateway/internal/healthcheck"
    "github.com/codyw912/ollama-gateway/internal/loadbalancer"
    "github.com/codyw912/ollama-gateway/internal/proxy"
    "github.com/codyw912/ollama-gateway/internal/server"
    "github.com/codyw912/ollama-gateway/pkg/models"
)

func main() {
    // Load configuration
    cfg, err := config.LoadConfig("config.json")
    if err != nil {
        log.Fatalf("Error loading config: %v\n", err)
    }

    // Initialize servers
    var servers []*models.OllamaServer
    for _, s := range cfg.Servers {
        servers = append(servers, &models.OllamaServer{
            Address:   s.Address,
            Priority:  s.Priority,
            IsHealthy: true,
        })
    }

    // Start health checks
    healthChecker := healthcheck.NewHealthChecker(servers)
    healthChecker.StartHealthChecks()

    // Initialize load balancer
    lb := loadbalancer.NewLoadBalancer(servers)

    // Initialize proxy
    p := proxy.NewProxy(lb)

    // Start server
    srv := server.NewServer(p, cfg.Port)
    if err := srv.Start(); err != nil {
        log.Fatalf("Could not start server: %v\n", err)
    }
}

