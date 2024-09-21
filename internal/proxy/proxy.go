package proxy

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/http/httputil"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/codyw912/ollama-gateway/internal/loadbalancer"
	"github.com/codyw912/ollama-gateway/pkg/models"

	"github.com/ollama/ollama/api"
)

type Proxy struct {
	LoadBalancer loadbalancer.LoadBalancer
}

func NewProxy(lb loadbalancer.LoadBalancer) *Proxy {
	return &Proxy{LoadBalancer: lb}
}

var endpointsRequiringModel = map[string]struct{}{
	"/generate":     {},
	"/chat":         {},
	"/embed":        {},
	"/api/generate": {},
	"/api/chat":     {},
	"/api/embed":    {},
	// Add more endpoints here if necessary
}

var aggregationEndpoints = map[string]struct{}{
	"/tags":     {},
	"/ps":       {},
	"/api/tags": {},
	"/api/ps":   {},
}

func normalizePath(path string) string {
	path = strings.TrimSpace(path)
	path = strings.TrimSuffix(path, "/") // Remove trailing slash
	return path
}

func requiresModelExtraction(path string) bool {
	_, exists := endpointsRequiringModel[path]
	return exists
}

func requiresAggregation(path string) bool {
	path = normalizePath(path)
	_, exists := aggregationEndpoints[path]
	return exists
}

func (p *Proxy) Handler(w http.ResponseWriter, r *http.Request) {
	var modelName string

	requestPath := normalizePath(r.URL.Path)
	log.Printf("Request path: %s", requestPath)

	// Check if the endpoint requires aggregation
	if requiresAggregation(requestPath) {
		p.handleAggregation(w, r, requestPath)
		return
	}

	// Only attempt to extract the model name for specific endpoints
	if r.Method == http.MethodPost && requiresModelExtraction(requestPath) {
		// Read the body
		bodyBytes, err := io.ReadAll(r.Body)
		if err != nil {
			http.Error(w, "Failed to read request body", http.StatusBadRequest)
			return
		}

		// Restore the body so it can be read by the proxy
		r.Body = io.NopCloser(bytes.NewBuffer(bodyBytes))
		r.ContentLength = int64(len(bodyBytes))

		// Parse the model name
		var requestData map[string]interface{}
		if err := json.Unmarshal(bodyBytes, &requestData); err != nil {
			http.Error(w, "Invalid JSON in request body", http.StatusBadRequest)
			return
		}

		// Log the parsed request data
		log.Printf("Parsed request data: %+v", requestData)

		// Extract model name
		if mn, ok := requestData["model"].(string); ok {
			modelName = mn
		} else {
			http.Error(w, "Model name is required in the request body", http.StatusBadRequest)
			return
		}
	} else {
		// For endpoints that don't need model name extraction
		// may need more handling for other POST method endpoints that don't have the same params (i.e. some endpoints expect "name" rather than "model")
		modelName = ""
	}

	// Select the server that has the model
	server := p.LoadBalancer.SelectServer(modelName)
	if server == nil {
		log.Printf("Requested model name: %s", modelName)
		http.Error(w, "No available servers with the requested model", http.StatusServiceUnavailable)
		return
	}

	log.Printf("[Proxy] Routing request %s %s to server %s\n", r.Method, r.URL.Path, server.Address)

	targetURL, err := url.Parse(server.Address)
	if err != nil {
		http.Error(w, "Invalid server address", http.StatusInternalServerError)
		return
	}

	proxy := httputil.NewSingleHostReverseProxy(targetURL)

	proxy.Director = func(req *http.Request) {
		req.URL.Scheme = targetURL.Scheme
		req.URL.Host = targetURL.Host
		req.URL.Path = r.URL.Path
		req.URL.RawQuery = r.URL.RawQuery
		req.Header = r.Header
		req.Body = r.Body
		req.ContentLength = r.ContentLength
	}

	proxy.ModifyResponse = func(resp *http.Response) error {
		if resp.StatusCode >= 500 {
			server.Mutex.Lock()
			server.IsHealthy = false
			server.Mutex.Unlock()
		}
		return nil
	}

	proxy.ErrorHandler = func(w http.ResponseWriter, req *http.Request, err error) {
		log.Printf("Proxy error: %v\n", err)
		server.Mutex.Lock()
		server.IsHealthy = false
		server.Mutex.Unlock()
		http.Error(w, "Server error", http.StatusBadGateway)
	}

	proxy.ServeHTTP(w, r)
}

func aggregateResponses(requestPath string, responses chan []byte) ([]byte, error) {
	switch requestPath {
	case "/tags", "/api/tags":
		return aggregateTags(responses)
	case "/ps", "/api/ps":
		return aggregatePs(responses)
	default:
		return nil, fmt.Errorf("Unsupported aggregation endpoint: %s", requestPath)
	}
}

func aggregateTags(responses chan []byte) ([]byte, error) {
	type TagsResponse struct {
		Models []api.ListModelResponse `json:"models"`
	}

	modelMap := make(map[string]api.ListModelResponse)

	for resp := range responses {
		var tagsResp TagsResponse
		if err := json.Unmarshal(resp, &tagsResp); err != nil {
			return nil, fmt.Errorf("Failed to parse tags response: %v", err)
		}

		for _, model := range tagsResp.Models {
			// Use the model name as the key to prevent duplicates
			modelMap[model.Name] = model
		}
	}

	// Convert the map back to a slice
	aggregatedModels := make([]api.ListModelResponse, 0, len(modelMap))
	for _, model := range modelMap {
		aggregatedModels = append(aggregatedModels, model)
	}

	// Marshal the aggregated data back to JSON
	aggregatedData, err := json.Marshal(TagsResponse{Models: aggregatedModels})
	if err != nil {
		return nil, fmt.Errorf("Failed to marshal aggregated tags: %v", err)
	}

	return aggregatedData, nil
}

func aggregatePs(responses chan []byte) ([]byte, error) {
	var allProcesses []api.ProcessModelResponse

	for resp := range responses {
		var psResp api.ProcessResponse
		if err := json.Unmarshal(resp, &psResp); err != nil {
			return nil, fmt.Errorf("Failed to parse ps response: %v", err)
		}

		allProcesses = append(allProcesses, psResp.Models...)
	}

	// Create a combined ProcessResponse with all processes
	aggregatedResponse := api.ProcessResponse{Models: allProcesses}

	// Marshal the aggregated data back to JSON
	aggregatedData, err := json.Marshal(aggregatedResponse)
	if err != nil {
		return nil, fmt.Errorf("Failed to marshal aggregated ps: %v", err)
	}

	return aggregatedData, nil
}

func (p *Proxy) handleAggregation(w http.ResponseWriter, r *http.Request, requestPath string) {
	// Get all healthy servers
	servers := p.LoadBalancer.GetAllHealthyServers()

	if len(servers) == 0 {
		http.Error(w, "No healthy servers available", http.StatusServiceUnavailable)
		return
	}

	// Channels to collect responses and errors
	responses := make(chan []byte, len(servers))
	errorsChan := make(chan error, len(servers))

	// WaitGroup to synchronize goroutines
	var wg sync.WaitGroup

	// Send requests to all servers
	for _, server := range servers {
		wg.Add(1)
		go func(s *models.OllamaServer) {
			defer wg.Done()

			// Build the target URL
			targetURL, err := url.Parse(s.Address)
			if err != nil {
				errorsChan <- fmt.Errorf("Invalid server address %s: %v", s.Address, err)
				return
			}
			targetURL.Path = r.URL.Path
			targetURL.RawQuery = r.URL.RawQuery

			// Create a new HTTP request
			req, err := http.NewRequest(r.Method, targetURL.String(), nil)
			if err != nil {
				errorsChan <- fmt.Errorf("Failed to create request for server %s: %v", s.Address, err)
				return
			}

			// Copy headers
			req.Header = r.Header.Clone()

			// Send the request
			client := &http.Client{Timeout: 5 * time.Second}
			resp, err := client.Do(req)
			if err != nil {
				errorsChan <- fmt.Errorf("Request to server %s failed: %v", s.Address, err)
				return
			}
			defer resp.Body.Close()

			if resp.StatusCode != http.StatusOK {
				errorsChan <- fmt.Errorf("Server %s returned status %d", s.Address, resp.StatusCode)
				return
			}

			body, err := io.ReadAll(resp.Body)
			if err != nil {
				errorsChan <- fmt.Errorf("Failed to read response from server %s: %v", s.Address, err)
				return
			}

			responses <- body
		}(server)
	}

	// Wait for all goroutines to finish
	wg.Wait()
	close(responses)
	close(errorsChan)

	// Collect errors
	var errs []string
	for err := range errorsChan {
		log.Println(err)
		errs = append(errs, err.Error())
	}

	// Decide how to handle partial failures
	if len(errs) == len(servers) {
		// All servers failed
		http.Error(w, "All servers failed to respond", http.StatusBadGateway)
		return
	}

	// Aggregate responses
	aggregatedData, err := aggregateResponses(requestPath, responses)
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to aggregate responses: %v", err), http.StatusInternalServerError)
		return
	}

	// Send the aggregated response to the client
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	w.Write(aggregatedData)
}
