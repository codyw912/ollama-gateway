package models

import (
	"github.com/ollama/ollama/api"
	"sync"
	"time"
)

type OllamaServer struct {
	Address     string
	Priority    int
	IsHealthy   bool
	LastChecked time.Time
	Models      []api.ListModelResponse
	Mutex       sync.RWMutex
}
