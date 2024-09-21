package loadbalancer

import (
	"github.com/codyw912/ollama-gateway/pkg/models"
	"log"
	"math"
	"strings"
)

type LoadBalancer interface {
	SelectServer(modelName string) *models.OllamaServer
	GetAllHealthyServers() []*models.OllamaServer
}

type priorityLoadBalancer struct {
	Servers []*models.OllamaServer
}

func NewLoadBalancer(servers []*models.OllamaServer) LoadBalancer {
	return &priorityLoadBalancer{Servers: servers}
}

func (lb *priorityLoadBalancer) SelectServer(modelName string) *models.OllamaServer {
	var selectedServer *models.OllamaServer
	minPriority := math.MaxInt32

	// Handle case where modelName is empty, some endpoints won't take model as param
	if modelName == "" {
		// Select any healthy server with the lowest priority
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
			log.Printf("[LoadBalancer] Selected server %s with priority %d (no specific model required)\n", selectedServer.Address, selectedServer.Priority)
		} else {
			log.Println("[LoadBalancer] No healthy servers available")
		}

		return selectedServer
	}

	// Normalize the requested model name
	requestedModel := modelName
	if !strings.Contains(modelName, ":") {
		requestedModel = modelName + ":latest"
	}

	for _, server := range lb.Servers {
		server.Mutex.RLock()
		isHealthy := server.IsHealthy
		priority := server.Priority
		hasModel := false
		for _, model := range server.Models {
			// Normalize server model name
			serverModelName := model.Name
			if !strings.Contains(model.Name, ":") {
				serverModelName = model.Name + ":latest"
			}

			if serverModelName == requestedModel {
				hasModel = true
				break
			}
		}
		server.Mutex.RUnlock()

		if isHealthy && hasModel && priority < minPriority {
			selectedServer = server
			minPriority = priority
		}
	}

	if selectedServer != nil {
		log.Printf("[LoadBalancer] Selected server %s with priority %d for model %s\n", selectedServer.Address, selectedServer.Priority, requestedModel)
	} else {
		log.Printf("[LoadBalancer] No healthy servers with model %s available\n", requestedModel)
	}

	return selectedServer
}

func (lb *priorityLoadBalancer) GetAllHealthyServers() []*models.OllamaServer {
	var healthyServers []*models.OllamaServer
	for _, server := range lb.Servers {
		server.Mutex.RLock()
		if server.IsHealthy {
			healthyServers = append(healthyServers, server)
		}
		server.Mutex.RUnlock()
	}
	return healthyServers
}
