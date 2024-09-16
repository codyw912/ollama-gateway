package models

import (
	"sync"
	"time"
  "github.com/ollama/ollama/api"
)

type OllamaServer struct {
	Address     string
	Priority    int
	IsHealthy   bool
	LastChecked time.Time
  Models      []api.ListModelResponse
	Mutex       sync.RWMutex
}
