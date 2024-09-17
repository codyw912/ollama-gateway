package loadbalancer

import (
  "github.com/codyw912/ollama-gateway/pkg/models"
  "log"
  "math"
)

type LoadBalancer interface {
    SelectServer() *models.OllamaServer
}

type priorityLoadBalancer struct {
    Servers []*models.OllamaServer
}

func NewLoadBalancer(servers []*models.OllamaServer) LoadBalancer {
    return &priorityLoadBalancer{Servers: servers}
}

func (lb *priorityLoadBalancer) SelectServer() *models.OllamaServer {
    var selectedServer *models.OllamaServer
    minPriority := math.MaxInt32

    for _, server := range lb.Servers {
        server.Mutex.RLock()
        isHealthy := server.IsHealthy
        priority := server.Priority
        server.Mutex.RUnlock()

        if isHealthy && priority < minPriority {
            selectedServer = server
            minPriority = priority
        }
    }

    if selectedServer != nil {
        log.Printf("[LoadBalancer] Selected server %s with priority %d\n", selectedServer.Address, selectedServer.Priority)
    } else {
        log.Println("[LoadBalancer] No healthy servers available")
    }

    return selectedServer
}

