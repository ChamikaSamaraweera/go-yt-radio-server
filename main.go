package main

import (
	"bytes"
	"io"
	"log"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"strconv"

	"github.com/joho/godotenv"
)

func getConfig() (host, port, ytDlpPath, ffmpegPath, cookiesPath string) {
	_ = godotenv.Load()
	host = getEnv("HOST", "")
	port = getEnv("PORT", "8080")
	ytDlpPath = getEnv("YT_DLP_PATH", "yt-dlp")
	ffmpegPath = getEnv("FFMPEG_PATH", "ffmpeg")
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

func listFormats(ytDlpPath, cookiesPath, vidURL string) string {
	args := []string{"--list-formats"}
	if cookiesPath != "" {
		args = append(args, "--cookies", cookiesPath)
	}
	args = append(args, vidURL)
	
	cmd := exec.Command(ytDlpPath, args...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return "Failed to list formats: " + err.Error()
	}
	return string(output)
}

func streamHandler(ytDlpPath, ffmpegPath, cookiesPath string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		vidURL := r.URL.Query().Get("url")
		debug := r.URL.Query().Get("debug")
		
		if vidURL == "" {
			http.Error(w, "Missing ?url=...", http.StatusBadRequest)
			return
		}
		if !isYouTubeURL(vidURL) {
			http.Error(w, "Only YouTube/YouTube Music URLs allowed", http.StatusBadRequest)
			return
		}

		if debug == "1" || debug == "true" {
			w.Header().Set("Content-Type", "text/plain")
			formats := listFormats(ytDlpPath, cookiesPath, vidURL)
			w.Write([]byte("=== Available Formats ===\n\n"))
			w.Write([]byte(formats))
			return
		}

		w.Header().Set("Content-Type", "audio/mpeg")
		w.Header().Set("Cache-Control", "no-cache")
		w.Header().Set("Connection", "keep-alive")
		w.Header().Set("Accept-Ranges", "none")
		w.Header().Set("Transfer-Encoding", "chunked")

		ctx := r.Context()

		log.Printf("🔍 Starting stream for: %s", vidURL)

		// Build yt-dlp command
		ytdlpArgs := []string{
			"-f", "worst",
			"-o", "-",
			"--no-warnings",
		}
		
		if cookiesPath != "" {
			ytdlpArgs = append(ytdlpArgs, "--cookies", cookiesPath)
		}
		ytdlpArgs = append(ytdlpArgs, vidURL)

		ytdlpCmd := exec.CommandContext(ctx, ytDlpPath, ytdlpArgs...)
		
		// Build ffmpeg command
		ffmpegCmd := exec.CommandContext(ctx, ffmpegPath,
			"-i", "pipe:0",
			"-vn",
			"-f", "mp3",
			"-ac", "2",
			"-ar", "44100",
			"-b:a", "128k",
			"-loglevel", "warning",
			"pipe:1",
		)
		
		// Connect yt-dlp stdout to ffmpeg stdin
		ytdlpStdout, err := ytdlpCmd.StdoutPipe()
		if err != nil {
			log.Printf("❌ yt-dlp pipe error: %v", err)
			http.Error(w, "Setup failed", http.StatusInternalServerError)
			return
		}
		ffmpegCmd.Stdin = ytdlpStdout
		
		// Capture stderr
		var ytdlpStderr, ffmpegStderr bytes.Buffer
		ytdlpCmd.Stderr = &ytdlpStderr
		ffmpegCmd.Stderr = &ffmpegStderr

		// Get ffmpeg stdout
		ffmpegStdout, err := ffmpegCmd.StdoutPipe()
		if err != nil {
			log.Printf("❌ ffmpeg pipe error: %v", err)
			http.Error(w, "ffmpeg setup failed", http.StatusInternalServerError)
			return
		}

		// Start yt-dlp
		if err := ytdlpCmd.Start(); err != nil {
			log.Printf("❌ yt-dlp start failed: %v", err)
			http.Error(w, "yt-dlp failed", http.StatusServiceUnavailable)
			return
		}
		
		// Start ffmpeg
		if err := ffmpegCmd.Start(); err != nil {
			ytdlpCmd.Process.Kill()
			log.Printf("❌ ffmpeg start failed: %v", err)
			http.Error(w, "ffmpeg failed", http.StatusInternalServerError)
			return
		}

		log.Printf("🎵 Streaming: %s", vidURL)

		// Cleanup on context cancel
		go func() {
			<-ctx.Done()
			ytdlpCmd.Process.Kill()
			ffmpegCmd.Process.Kill()
			log.Printf("🛑 Stream stopped: %s", vidURL)
		}()

		// Stream data to client
		buf := make([]byte, 64*1024)
		flusher, _ := w.(http.Flusher)
		total := 0

		for {
			n, err := ffmpegStdout.Read(buf)
			if n > 0 {
				total += n
				if _, e := w.Write(buf[:n]); e != nil {
					log.Printf("Client left after %d bytes", total)
					ytdlpCmd.Process.Kill()
					ffmpegCmd.Process.Kill()
					return
				}
				if flusher != nil {
					flusher.Flush()
				}
			}
			if err != nil {
				if err != io.EOF {
					log.Printf("⚠️ Read error: %v", err)
				}
				break
			}
		}

		// Wait for processes to finish
		ffmpegCmd.Wait()
		ytdlpCmd.Wait()
		
		// Log any errors
		if ytdlpStderr.Len() > 0 {
			log.Printf("yt-dlp stderr: %s", ytdlpStderr.String())
		}
		if ffmpegStderr.Len() > 0 {
			log.Printf("ffmpeg stderr: %s", ffmpegStderr.String())
		}
		
		if total == 0 {
			log.Printf("❌ WARNING: Stream completed but sent 0 bytes!")
		}
		
		log.Printf("✅ Stream done: %s (%d bytes)", vidURL, total)
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
	host, port, ytDlpPath, ffmpegPath, cookiesPath := getConfig()
	addr := net.JoinHostPort(host, port)

	http.HandleFunc("/radio/stream", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
		if r.Method == "OPTIONS" {
			w.WriteHeader(http.StatusOK)
			return
		}
		streamHandler(ytDlpPath, ffmpegPath, cookiesPath)(w, r)
	})

	http.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Access-Control-Allow-Origin", "*")
		cookieStatus := "none"
		if cookiesPath != "" {
			cookieStatus = cookiesPath
		}
		w.Write([]byte(`{"status":"ok","yt_dlp":"` + ytDlpPath + `","ffmpeg":"` + ffmpegPath + `","cookies":"` + cookieStatus + `"}`))
	})

	log.Printf("📻 Radio Server listening on http://%s", addr)
	log.Printf("⚙️ Using yt-dlp: %s | ffmpeg: %s", ytDlpPath, ffmpegPath)
	if cookiesPath != "" {
		log.Printf("🍪 Using cookies: %s", cookiesPath)
	} else {
		log.Printf("🍪 No cookies configured")
	}
	log.Printf("🔍 Debug mode: Add ?debug=1 to see available formats")
	
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