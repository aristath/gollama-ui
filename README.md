# Gollama UI

A minimal, lightweight web UI for Ollama built with Go. Designed for low-resource environments like Raspberry Pi.

## Features

- **Chat Interface**: Clean, minimal chat UI with streaming responses
- **Model Switching**: Switch between available Ollama models from the UI
- **Low Resource Usage**: ~20MB RAM, perfect for Raspberry Pi
- **Streaming**: Real-time streaming responses using Server-Sent Events (SSE)
- **Simple Architecture**: Clean Go architecture with separation of concerns

## Requirements

- Go 1.21 or later
- Ollama installed and running (default: `http://localhost:11434`)
- At least one Ollama model installed

## Installation

1. Clone the repository:
```bash
git clone git@github.com:aristath/gollama-ui.git
cd gollama-ui
```

2. Install dependencies:
```bash
go mod download
```

3. Build the server:
```bash
go build -o gollama-ui ./cmd/server
```

## Usage

### Basic Usage

Run the server with default settings:
```bash
./gollama-ui
```

This will:
- Start the server on `http://0.0.0.0:8080`
- Connect to Ollama at `http://localhost:11434`
- Serve static files from `./web`

### Command Line Options

```bash
./gollama-ui -help
```

Options:
- `-host`: Server host (default: `0.0.0.0`)
- `-port`: Server port (default: `8080`)
- `-ollama`: Ollama server URL (default: `http://localhost:11434`)
- `-static`: Static files directory (default: `./web`)

### Example: Custom Configuration

```bash
./gollama-ui \
  -host 192.168.1.17 \
  -port 3000 \
  -ollama http://localhost:11434 \
  -static ./web
```

## Architecture

```
gollama-ui/
├── cmd/server/          # Application entry point
│   └── main.go
├── internal/
│   ├── client/          # Ollama client wrapper
│   │   └── ollama.go
│   ├── handlers/        # HTTP handlers
│   │   ├── models.go    # Model listing endpoint
│   │   └── chat.go      # Chat streaming endpoint
│   └── server/          # HTTP server setup
│       └── server.go
└── web/                 # Frontend static files
    ├── index.html
    ├── styles.css
    └── app.js
```

### Design Principles

- **Clean Architecture**: Separation of concerns with client, handlers, and server layers
- **Interface-Based**: Handlers depend on interfaces, not concrete types
- **Minimal Dependencies**: Only essential dependencies (chi router, ollama-go)
- **Streaming-First**: Built for real-time chat with SSE

## API Endpoints

### GET /api/models

Returns a list of available Ollama models.

**Response:**
```json
{
  "models": [
    {
      "name": "llama3.2",
      "size": 2147483648,
      "digest": "sha256:...",
      "modified_at": "2024-01-01T00:00:00Z"
    }
  ]
}
```

### POST /api/chat

Sends a chat message and streams the response.

**Request:**
```json
{
  "model": "llama3.2",
  "messages": [
    {
      "role": "user",
      "content": "Hello, how are you?"
    }
  ],
  "stream": true
}
```

**Response:** Server-Sent Events (SSE) stream:
```
data: {"model":"llama3.2","message":{"role":"assistant","content":"Hello"},"done":false}
data: {"model":"llama3.2","message":{"role":"assistant","content":"! How"},"done":false}
data: {"model":"llama3.2","message":{"role":"assistant","content":" can I help"},"done":false}
data: {"model":"llama3.2","message":{"role":"assistant","content":" you?"},"done":true}
```

## Development

### Building

```bash
go build -o gollama-ui ./cmd/server
```

### Running Tests

```bash
go test ./...
```

### Running Locally

1. Ensure Ollama is running:
```bash
ollama serve
```

2. Pull a model (if needed):
```bash
ollama pull llama3.2
```

3. Run the server:
```bash
go run ./cmd/server
```

4. Open `http://localhost:8080` in your browser

## Troubleshooting

### "Failed to load models"

- Ensure Ollama is running: `ollama list`
- Check Ollama URL matches your setup (default: `http://localhost:11434`)
- Verify at least one model is installed: `ollama list`

### "Streaming not supported"

- Ensure you're using a compatible HTTP server (the built-in Go server supports it)
- If behind a proxy (nginx), ensure it's configured to pass SSE streams

### Port Already in Use

- Use a different port: `./gollama-ui -port 8081`
- Or stop the service using port 8080

## License

MIT

## Contributing

Contributions welcome! Please ensure:
- Code follows Go best practices
- Architecture remains clean and minimal
- Resource usage is kept low
- Tests are included for new features