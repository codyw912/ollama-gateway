package healthcheck

import (
    "encoding/json"
    "log"
    "net/http"
    "time"

    "github.com/ollama/ollama/api"
    "github.com/codyw912/ollama-gateway/pkg/models"
)

type HealthChecker interface {
    StartHealthChecks()
}

type healthChecker struct {
    Servers []*models.OllamaServer
}

func NewHealthChecker(servers []*models.OllamaServer) HealthChecker {
    return &healthChecker{Servers: servers}
}

func (hc *healthChecker) StartHealthChecks() {
    for _, server := range hc.Servers {
        go hc.healthCheck(server)
    }
}

func (hc *healthChecker) healthCheck(server *models.OllamaServer) {
    client := &http.Client{Timeout: 5 * time.Second}
    for {
        resp, err := client.Get(server.Address + "/api/tags")
        server.Mutex.Lock()
        if err != nil {
            log.Printf("Server %s is unhealthy: %v\n", server.Address, err)
            server.IsHealthy = false
            server.Models = nil
        } else if resp.StatusCode != http.StatusOK {
            log.Printf("Server %s returned status: %d\n", server.Address, resp.StatusCode)
            server.IsHealthy = false
            server.Models = nil
        } else {
            var tagsResponse api.ListResponse
            decoder := json.NewDecoder(resp.Body)
            if err := decoder.Decode(&tagsResponse); err != nil {
                log.Printf("Server %s invalid JSON: %v\n", server.Address, err)
                server.IsHealthy = false
                server.Models = nil
            } else {
                server.IsHealthy = true
                server.Models = tagsResponse.Models
            }
        }
        server.LastChecked = time.Now()
        server.Mutex.Unlock()
        if resp != nil {
            resp.Body.Close()
        }
        time.Sleep(10 * time.Second)
    }
}
