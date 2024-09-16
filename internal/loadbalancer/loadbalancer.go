package loadbalancer

import "github.com/codyw912/ollama-gateway/pkg/models"

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
    maxPriority := -1

    for _, server := range lb.Servers {
        server.Mutex.RLock()
        isHealthy := server.IsHealthy
        priority := server.Priority
        server.Mutex.RUnlock()

        if isHealthy && priority > maxPriority {
            selectedServer = server
            maxPriority = priority
        }
    }
    return selectedServer
}

