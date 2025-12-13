package main

import (
	"log"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"strconv"

	"github.com/joho/godotenv"
)

func getConfig() (host, port, ytDlpPath, cookiesPath string) {
	_ = godotenv.Load()
	host = getEnv("HOST", "")
	port = getEnv("PORT", "8080")
	ytDlpPath = getEnv("YT_DLP_PATH", "yt-dlp")
	cookiesPath = getEnv("COOKIES_PATH", "")
	if _, err := strconv.Atoi(port); err != nil {
		log.Printf("⚠️ Invalid PORT='%s', using 8080", port)
		port = "8080"
	}
	return
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func streamHandler(ytDlpPath, cookiesPath string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
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
		w.Header().Set("Accept-Ranges", "none")
		w.Header().Set("Transfer-Encoding", "chunked")

		ctx := r.Context()

		// Robust 2025 YouTube format selection
		ytdlpArgs := []string{
			"--extractor-args", "youtube:player_client=web,ios,android;include_live_dash",
			"--compat-options", "no-youtube-unavailable-videos",
			"-f", "best[ext=mp4][acodec~='mp4a']/best[ext=mp4]/best",
			"--extract-audio",
			"--audio-format", "mp3",
			"--audio-quality", "0",
			"-o", "-",
			"--quiet",
			"--no-warnings",
		}

		if cookiesPath != "" {
			ytdlpArgs = append(ytdlpArgs, "--cookies", cookiesPath)
		}
		ytdlpArgs = append(ytdlpArgs, vidURL)

		cmd := exec.CommandContext(ctx, ytDlpPath, ytdlpArgs...)
		stdout, err := cmd.StdoutPipe()
		if err != nil {
			log.Printf("pipe error: %v", err)
			http.Error(w, "Internal error", http.StatusInternalServerError)
			return
		}

		stderr, _ := cmd.StderrPipe()
		go func() {
			buf := make([]byte, 1024)
			for {
				n, err := stderr.Read(buf)
				if n > 0 {
					log.Printf("yt-dlp: %s", string(buf[:n]))
				}
				if err != nil {
					break
				}
			}
		}()

		if err := cmd.Start(); err != nil {
			log.Printf("yt-dlp start failed: %v", err)
			http.Error(w, "yt-dlp failed", http.StatusServiceUnavailable)
			return
		}

		go func() {
			<-ctx.Done()
			cmd.Process.Kill()
			log.Printf("🛑 Stream stopped: %s", vidURL)
		}()

		buf := make([]byte, 64*1024)
		flusher, _ := w.(http.Flusher)
		total := 0

		for {
			n, err := stdout.Read(buf)
			if n > 0 {
				total += n
				if _, e := w.Write(buf[:n]); e != nil {
					log.Printf("Client left after %d bytes", total)
					cmd.Process.Kill()
					return
				}
				if flusher != nil {
					flusher.Flush()
				}
			}
			if err != nil {
				break
			}
		}

		cmd.Wait()
		log.Printf("✅ Stream done: %d bytes", total)
	}
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
	host, port, ytDlpPath, cookiesPath := getConfig()
	addr := net.JoinHostPort(host, port)

	http.HandleFunc("/radio/stream", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
		if r.Method == "OPTIONS" {
			w.WriteHeader(http.StatusOK)
			return
		}
		streamHandler(ytDlpPath, cookiesPath)(w, r)
	})

	http.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Access-Control-Allow-Origin", "*")
		cookieStatus := "none"
		if cookiesPath != "" {
			cookieStatus = cookiesPath
		}
		w.Write([]byte(`{"status":"ok","yt_dlp":"` + ytDlpPath + `","cookies":"` + cookieStatus + `"}`))
	})

	log.Printf("📻 Radio Server listening on http://%s", addr)
	log.Printf("⚙️ Using yt-dlp: %s", ytDlpPath)
	if cookiesPath != "" {
		log.Printf("🍪 Using cookies: %s", cookiesPath)
	} else {
		log.Printf("🍪 No cookies configured")
	}
	if host == "" || host == "0.0.0.0" {
		if ip := getOutboundIP(); ip != "" {
			log.Printf("🌐 LAN access: http://%s:%s/radio/stream?url=...", ip, port)
		}
	}
	log.Fatal(http.ListenAndServe(addr, nil))
}

func getOutboundIP() string {
	conn, err := net.Dial("udp", "8.8.8.8:80")
	if err != nil {
		return ""
	}
	defer conn.Close()
	return conn.LocalAddr().(*net.UDPAddr).IP.String()
}