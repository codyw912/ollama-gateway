# Ollama Gateway

A Go-based gateway that provides a unified interface to multiple Ollama servers, seamlessly routing client requests.

## Usage
- Copy the example config file and rename to `config.json`
```bash
cp config.example.json config.json
```
- Replace example servers with your actual server URLs. If running on your local machine, Ollama's default is `http://localhost:11434`

- Test it out with
```bash
go run cmd/gateway/main.go
```

- You can send requests to the gateway as you would to a single Ollama server:
```bash
curl http://localhost:9090/api/chat -d '{                                                                                                                                                                         
  "model": "llama3.1",
  "messages": [
    {
      "role": "user",
      "content": "why is the sky blue?"
    }
  ]
}'
```
