package config

import (
    "encoding/json"
    "log"
    "os"
)

type ServerConfig struct {
    Address  string `json:"address"`
    Priority int    `json:"priority"`
}

type Config struct {
    Servers []ServerConfig `json:"servers"`
    Port    string         `json:"port"`
}

func LoadConfig(filePath string) (*Config, error) {
    data, err := os.ReadFile(filePath)
    if err != nil {
        log.Printf("Error reading config file: %v\n", err)
        return nil, err
    }

    var config Config
    if err := json.Unmarshal(data, &config); err != nil {
        log.Printf("Error parsing config file: %v\n", err)
        return nil, err
    }

    return &config, nil
}
