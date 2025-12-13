# 📻 Go YouTube Radio Server

> A lightweight HTTP streaming server that converts YouTube/YouTube Music URLs to real-time **MP3 audio streams** — perfect for **MTA:SA radio mods**, game servers, and personal streaming projects.

[![Go](https://img.shields.io/badge/Go-1.23+-00ADD8?logo=go)](https://go.dev)
[![License](https://img.shields.io/badge/License-MIT-blue.svg)](LICENSE)
[![Release](https://img.shields.io/github/v/release/ChamikaSamaraweera/go-yt-radio-server)](https://github.com/ChamikaSamaraweera/go-yt-radio-server/releases)
[![Platform](https://img.shields.io/badge/Platform-Linux%20ARM64%20%7C%20AMD64-green)]()

⚠️ **For personal/private use only** — This server does **not** comply with YouTube's Terms of Service for public hosting. Use responsibly on your own network.

---

## 🎯 Features

- ✅ **Real-time Streaming** - YouTube → MP3 conversion on-the-fly (no files saved)
- ✅ **MTA:SA Compatible** - Tested with MTA: San Andreas radio systems
- ✅ **Optimized Audio** - 44.1kHz stereo MP3 @ 128kbps
- ✅ **Minimal Footprint** - Single binary, <10MB RAM usage
- ✅ **Auto-Cleanup** - Streams terminate on client disconnect
- ✅ **Cookie Support** - Handle age-restricted & premium content
- ✅ **Easy Config** - Environment variables via `.env`
- ✅ **Production Ready** - Docker & Pterodactyl compatible

---

## 📋 Requirements

### Server Requirements
- **yt-dlp** - [Latest version](https://github.com/yt-dlp/yt-dlp/releases)
- **ffmpeg** - Static build recommended for containers
- **Go 1.23+** - For building from source

### Supported Platforms
- Linux ARM64 (Raspberry Pi, Oracle ARM instances)
- Linux AMD64 (Standard servers)
- Docker containers (tested with Pterodactyl)

---

## 🚀 Quick Start

### Option 1: Download Pre-built Binary

```bash
# For ARM64 (Raspberry Pi, Oracle Cloud ARM)
curl -L https://github.com/ChamikaSamaraweera/go-yt-radio-server/releases/latest/download/radio-server-linux-arm64 -o radio-server
chmod +x radio-server

# For AMD64 (Standard Linux)
curl -L https://github.com/ChamikaSamaraweera/go-yt-radio-server/releases/latest/download/radio-server-linux-amd64 -o radio-server
chmod +x radio-server
```

### Option 2: Build from Source

```bash
# Clone repository
git clone https://github.com/ChamikaSamaraweera/go-yt-radio-server.git
cd go-yt-radio-server

# Build for your platform
go build -o radio-server main.go

# Or cross-compile for ARM64
GOOS=linux GOARCH=arm64 go build -o radio-server-linux-arm64 main.go
```

---

## ⚙️ Installation

### 1. Install Dependencies

#### For Standard Linux:
```bash
# Install ffmpeg
sudo apt update
sudo apt install -y ffmpeg python3 python3-pip

# Install yt-dlp
sudo pip3 install -U yt-dlp
```

#### For Docker/Pterodactyl (Static Binaries):
```bash
# Download static ffmpeg for ARM64
wget https://johnvansickle.com/ffmpeg/releases/ffmpeg-release-arm64-static.tar.xz
tar -xf ffmpeg-release-arm64-static.tar.xz
cp ffmpeg-*-arm64-static/ffmpeg ./

# Download yt-dlp
curl -L https://github.com/yt-dlp/yt-dlp/releases/latest/download/yt-dlp -o yt-dlp
chmod +x yt-dlp
```

### 2. Configure Environment

Create a `.env` file:

```env
# Server Configuration
HOST=0.0.0.0          # Listen on all interfaces (use specific IP for security)
PORT=8194             # Port to listen on

# Binary Paths
YT_DLP_PATH=/usr/local/bin/yt-dlp    # Path to yt-dlp binary
FFMPEG_PATH=/usr/bin/ffmpeg           # Path to ffmpeg binary

# Optional: Cookie file for age-restricted content
COOKIES_PATH=./cookies.txt
```

### 3. Run the Server

```bash
./radio-server
```

Expected output:
```
📻 Radio Server listening on http://0.0.0.0:8194
⚙️ Using yt-dlp: /usr/local/bin/yt-dlp | ffmpeg: /usr/bin/ffmpeg
🍪 No cookies configured (set COOKIES_PATH if needed)
🌐 LAN access: http://192.168.1.100:8194/radio/stream?url=...
```

---

## 🎮 Usage

### Basic Streaming

```bash
# Stream a YouTube video
curl "http://localhost:8194/radio/stream?url=https://youtu.be/dQw4w9WgXcQ" | mpg123 -
```

### MTA:SA Integration

```lua
-- In your MTA:SA script
local serverURL = "http://your-server-ip:8194"
local youtubeURL = "https://youtu.be/dQw4w9WgXcQ"
local streamURL = serverURL .. "/radio/stream?url=" .. youtubeURL

-- Play the stream
local sound = playSound(streamURL)
```

### Web Player

```html
<!DOCTYPE html>
<html>
<body>
    <audio controls autoplay>
        <source src="http://localhost:8194/radio/stream?url=https://youtu.be/dQw4w9WgXcQ" type="audio/mpeg">
    </audio>
</body>
</html>
```

### Health Check

```bash
curl http://localhost:8194/health
```

Response:
```json
{"status":"ok","yt_dlp":"/usr/local/bin/yt-dlp","ffmpeg":"/usr/bin/ffmpeg","cookies":"none"}
```

---

## 🍪 Cookie Support

For age-restricted or region-locked videos, provide YouTube cookies:

### Export Cookies from Browser

**Method 1: Browser Extension**
1. Install [Get cookies.txt LOCALLY](https://chrome.google.com/webstore/detail/get-cookiestxt-locally) (Chrome)
2. Visit youtube.com (logged in)
3. Export cookies → save as `cookies.txt`

**Method 2: yt-dlp Command**
```bash
yt-dlp --cookies-from-browser chrome --cookies cookies.txt "https://www.youtube.com/watch?v=dQw4w9WgXcQ"
```

Then update `.env`:
```env
COOKIES_PATH=./cookies.txt
```

---

## 🐳 Docker Deployment

### Using Docker Compose

```yaml
version: '3.8'
services:
  radio-server:
    image: golang:1.23-alpine
    working_dir: /app
    command: ./radio-server
    ports:
      - "8194:8194"
    volumes:
      - ./:/app
    environment:
      - HOST=0.0.0.0
      - PORT=8194
      - YT_DLP_PATH=/usr/local/bin/yt-dlp
      - FFMPEG_PATH=/usr/bin/ffmpeg
```

---

## 🎯 Pterodactyl Panel Setup

### 1. Upload Files
- `radio-server-linux-arm64` (or amd64)
- `yt-dlp` (pure Python version)
- `ffmpeg` (static binary)
- `cookies.txt` (optional)
- `.env`

### 2. Set Executable Permissions
```bash
chmod +x radio-server-linux-arm64
chmod +x yt-dlp
chmod +x ffmpeg
```

### 3. Update .env with Container Paths
```env
PORT=8194
YT_DLP_PATH=/home/container/yt-dlp
FFMPEG_PATH=/home/container/ffmpeg
COOKIES_PATH=/home/container/cookies.txt
```

### 4. Start Command
```bash
./radio-server-linux-arm64
```

---

## 🔧 Troubleshooting

### Stream Not Playing
**Problem:** 0 bytes sent, immediate disconnection

**Solutions:**
1. **Check yt-dlp works:**
   ```bash
   ./yt-dlp -f worst -o - "https://youtu.be/dQw4w9WgXcQ" | head -c 1000
   ```

2. **Check ffmpeg works:**
   ```bash
   ./ffmpeg -version
   ```

3. **Verify paths in `.env`** match actual binary locations

4. **Test manually:**
   ```bash
   ./yt-dlp -f worst -o - "URL" | ./ffmpeg -i pipe:0 -f mp3 -ac 2 -ar 44100 -b:a 128k -vn pipe:1 | head -c 10000
   ```

### PyInstaller Errors
**Problem:** `[PYI-xxx:ERROR] Failed to extract...`

**Solution:** Use pure Python yt-dlp instead of standalone binary
```bash
# Download pure Python version
curl -L https://github.com/yt-dlp/yt-dlp/releases/latest/download/yt-dlp -o yt-dlp
chmod +x yt-dlp
```

### Browser Autoplay Blocked
**Problem:** Stream loads but doesn't play

**Solution:** Create HTML player with user-triggered play button (see Usage section)

### DNS Resolution Errors
**Problem:** `Failed to resolve hostname manifest.googlevideo.com`

**Solution:** Use yt-dlp → ffmpeg pipeline (already implemented in latest version)

---

## 📊 Performance

- **CPU Usage:** ~5-10% during active streaming
- **Memory:** <10MB base, ~50MB during transcoding
- **Bandwidth:** ~128kbps per stream (MP3 bitrate)
- **Latency:** 5-10 seconds initial buffering

---

## 🛣️ Roadmap

- [ ] Playlist support
- [ ] Stream caching
- [ ] Multiple quality options
- [ ] WebSocket status updates
- [ ] Admin dashboard
- [ ] Prometheus metrics

---

## 🤝 Contributing

Contributions are welcome! Please:

1. Fork the repository
2. Create a feature branch (`git checkout -b feature/AmazingFeature`)
3. Commit your changes (`git commit -m 'Add some AmazingFeature'`)
4. Push to the branch (`git push origin feature/AmazingFeature`)
5. Open a Pull Request

---

## 📄 License

This project is licensed under the MIT License - see the [LICENSE](LICENSE) file for details.

---

## ⚠️ Disclaimer

This software is provided for **educational and personal use only**. 

- **YouTube Terms of Service:** Downloading or streaming YouTube content violates YouTube's Terms of Service unless explicitly permitted.
- **Copyright:** Respect copyright laws and content creators' rights.
- **Private Use:** This server is designed for personal, private networks only.
- **No Warranty:** Use at your own risk. The authors are not responsible for any misuse.

**By using this software, you agree to use it responsibly and in compliance with all applicable laws.**

---

## 💬 Support

- **Issues:** [GitHub Issues](https://github.com/ChamikaSamaraweera/go-yt-radio-server/issues)
- **Discussions:** [GitHub Discussions](https://github.com/ChamikaSamaraweera/go-yt-radio-server/discussions)

---

## 🙏 Acknowledgments

- [yt-dlp](https://github.com/yt-dlp/yt-dlp) - YouTube video/audio extraction
- [FFmpeg](https://ffmpeg.org/) - Media transcoding
- MTA:SA Community - Inspiration and testing

---

**Made with ❤️ for the MTA:SA community**