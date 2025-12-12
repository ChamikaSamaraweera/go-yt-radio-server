package main

import (
	"context"
	"log"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"strconv"
	"time"

	"github.com/joho/godotenv"
)

func getEnv(key, fallback string) string {
	if value, ok := os.LookupEnv(key); ok {
		return value
	}
	return fallback
}

func loadConfig() (host, port string) {
	// Try to load .env (ignores error if file not found)
	_ = godotenv.Load() // safe — returns error if .env missing, but we ignore it

	host = getEnv("HOST", "")
	port = getEnv("PORT", "8080")

	// Validate port
	if _, err := strconv.Atoi(port); err != nil {
		log.Printf("⚠️ Invalid PORT='%s', using 8080", port)
		port = "8080"
	}

	return
}

func streamHandler(w http.ResponseWriter, r *http.Request) {
	vidURL := r.URL.Query().Get("url")
	if vidURL == "" {
		http.Error(w, "Missing ?url=...", http.StatusBadRequest)
		return
	}

	if !isYouTubeURL(vidURL) {
		http.Error(w, "Only YouTube/YouTube Music URLs allowed", http.StatusBadRequest)
		return
	}

	w.Header().Set("Content-Type", "audio/mpeg")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")

	ctx, cancel := context.WithTimeout(r.Context(), 45*time.Second)
	defer cancel()

	ytdlpCmd := exec.CommandContext(ctx, "yt-dlp",
		"-f", "bestaudio[ext=m4a]/bestaudio",
		"-o", "-",
		"--no-warnings",
		"--quiet",
		vidURL,
	)

	ffmpegCmd := exec.CommandContext(ctx, "ffmpeg",
		"-hide_banner", "-loglevel", "error",
		"-i", "pipe:0",
		"-f", "mp3",
		"-ac", "2",
		"-ar", "44100",
		"-b:a", "128k",
		"-vn",
		"pipe:1",
	)

	ytdlpStdout, err := ytdlpCmd.StdoutPipe()
	if err != nil {
		log.Printf("yt-dlp pipe: %v", err)
		http.Error(w, "Internal error", http.StatusInternalServerError)
		return
	}
	ffmpegCmd.Stdin = ytdlpStdout

	if err := ytdlpCmd.Start(); err != nil {
		log.Printf("yt-dlp start: %v", err)
		http.Error(w, "Fetch failed", http.StatusServiceUnavailable)
		return
	}

	ffmpegStdout, err := ffmpegCmd.StdoutPipe()
	if err != nil {
		ytdlpCmd.Process.Kill()
		log.Printf("ffmpeg pipe: %v", err)
		http.Error(w, "Encoder setup failed", http.StatusInternalServerError)
		return
	}

	if err := ffmpegCmd.Start(); err != nil {
		ytdlpCmd.Process.Kill()
		log.Printf("ffmpeg start: %v", err)
		http.Error(w, "Encoding failed", http.StatusInternalServerError)
		return
	}

	// Kill on client disconnect
	go func() {
		<-r.Context().Done()
		ytdlpCmd.Process.Kill()
		ffmpegCmd.Process.Kill()
	}()

	buf := make([]byte, 64*1024)
	for {
		n, err := ffmpegStdout.Read(buf)
		if n > 0 {
			if _, writeErr := w.Write(buf[:n]); writeErr != nil {
				ytdlpCmd.Process.Kill()
				ffmpegCmd.Process.Kill()
				return
			}
			if f, ok := w.(http.Flusher); ok {
				f.Flush()
			}
		}
		if err != nil {
			break
		}
	}

	ffmpegCmd.Wait()
	ytdlpCmd.Wait()
}

func isYouTubeURL(u string) bool {
	parsed, err := url.Parse(u)
	if err != nil {
		return false
	}
	host := parsed.Hostname()
	return host == "youtube.com" ||
		host == "www.youtube.com" ||
		host == "youtu.be" ||
		host == "music.youtube.com"
}

func main() {
	host, port := loadConfig()
	addr := net.JoinHostPort(host, port)

	http.HandleFunc("/radio/stream", streamHandler)
	http.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"status":"ok"}`))
	})

	log.Printf("Radio Server listening on http://%s", addr)
	if host == "" || host == "0.0.0.0" {
		// Show LAN IP for convenience (optional)
		if ip := getOutboundIP(); ip != "" {
			log.Printf("LAN access: http://%s:%s", ip, port)
		}
	}
	log.Fatal(http.ListenAndServe(addr, nil))
}

// getOutboundIP tries to detect LAN IP (for dev convenience only)
func getOutboundIP() string {
	conn, err := net.Dial("udp", "8.8.8.8:80")
	if err != nil {
		return ""
	}
	defer conn.Close()
	localAddr := conn.LocalAddr().(*net.UDPAddr)
	return localAddr.IP.String()
}