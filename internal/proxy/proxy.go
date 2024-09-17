package proxy

import (
    "log"
    "net/http"
    "net/http/httputil"
    "net/url"

    "github.com/codyw912/ollama-gateway/internal/loadbalancer"
)

type Proxy struct {
    LoadBalancer loadbalancer.LoadBalancer
}

func NewProxy(lb loadbalancer.LoadBalancer) *Proxy {
    return &Proxy{LoadBalancer: lb}
}

func (p *Proxy) Handler(w http.ResponseWriter, r *http.Request) {
    server := p.LoadBalancer.SelectServer()
    if server == nil {
        http.Error(w, "No available servers", http.StatusServiceUnavailable)
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

