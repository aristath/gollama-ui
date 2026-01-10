# Deployment Guide for Raspberry Pi

## Prerequisites

- Raspberry Pi (tested on Pi 4 with 8GB RAM)
- Ollama installed and running
- SSH access to the Pi

## Quick Deployment

### Option 1: Build Locally and Transfer

1. **Build the ARM64 binary:**
   ```bash
   make build-arm64
   ```

2. **Transfer to Raspberry Pi:**
   ```bash
   scp gollama-ui-arm64 aristath@192.168.1.17:~/gollama-ui
   scp -r web/ aristath@192.168.1.17:~/gollama-ui/
   ```

3. **SSH into the Pi and run:**
   ```bash
   ssh aristath@192.168.1.17
   chmod +x ~/gollama-ui/gollama-ui-arm64
   cd ~/gollama-ui
   ./gollama-ui-arm64 -host 0.0.0.0 -port 8080
   ```

### Option 2: Build on Raspberry Pi

1. **Clone and build on the Pi:**
   ```bash
   ssh aristath@192.168.1.17
   git clone git@github.com:aristath/gollama-ui.git
   cd gollama-ui
   go build -o gollama-ui ./cmd/server
   ```

2. **Run:**
   ```bash
   ./gollama-ui -host 0.0.0.0 -port 8080
   ```

## Running as a Service (systemd)

1. **Create service file on Pi:**
   ```bash
   sudo nano /etc/systemd/system/gollama-ui.service
   ```

2. **Add this content:**
   ```ini
   [Unit]
   Description=Gollama UI - Ollama Web Interface
   After=network.target ollama.service
   Requires=ollama.service

   [Service]
   Type=simple
   User=aristath
   WorkingDirectory=/home/aristath/gollama-ui
   ExecStart=/home/aristath/gollama-ui/gollama-ui-arm64 -host 0.0.0.0 -port 8080
   Restart=always
   RestartSec=5

   [Install]
   WantedBy=multi-user.target
   ```

3. **Enable and start the service:**
   ```bash
   sudo systemctl daemon-reload
   sudo systemctl enable gollama-ui
   sudo systemctl start gollama-ui
   sudo systemctl status gollama-ui
   ```

## Verification

1. **Check if Ollama is running:**
   ```bash
   ollama list
   ```

2. **Check if gollama-ui is running:**
   ```bash
   curl http://localhost:8080/api/models
   ```

3. **Access the web UI:**
   Open `http://192.168.1.17:8080` in your browser

## Troubleshooting

### Port Already in Use
```bash
# Find what's using port 8080
sudo lsof -i :8080
# Or use a different port
./gollama-ui-arm64 -port 3000
```

### Ollama Not Accessible
```bash
# Check if Ollama is running
systemctl status ollama
# Check Ollama port
curl http://localhost:11434/api/tags
# If Ollama is on different host/port, specify it:
./gollama-ui-arm64 -ollama http://localhost:11434
```

### Permission Denied
```bash
chmod +x gollama-ui-arm64
```

### Static Files Not Found
Ensure the `web/` directory is in the same directory as the binary, or specify it:
```bash
./gollama-ui-arm64 -static /path/to/web
```

## Firewall Configuration

If you need to access from other devices on the network:

```bash
sudo ufw allow 8080/tcp
```

## Resource Usage

- **RAM**: ~20MB for the Go server
- **CPU**: Minimal (mostly idle)
- **Disk**: Binary is ~10MB, web files are <100KB

## Updating

```bash
cd ~/gollama-ui
git pull
go build -o gollama-ui-arm64 ./cmd/server
sudo systemctl restart gollama-ui
```