package main

import (
	"context"
	"log"
	"net/http"
	"net/url"
	"os/exec"
	"time"
)

func streamHandler(w http.ResponseWriter, r *http.Request) {
	vidURL := r.URL.Query().Get("url")
	if vidURL == "" {
		http.Error(w, "Missing ?url=...", http.StatusBadRequest)
		return
	}

	// Basic validation
	if !isYouTubeURL(vidURL) {
		http.Error(w, "Only YouTube/YouTube Music URLs allowed", http.StatusBadRequest)
		return
	}

	// Set headers for streaming MP3
	w.Header().Set("Content-Type", "audio/mpeg")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")

	// Build yt-dlp + ffmpeg pipeline:
	// yt-dlp → stdout → ffmpeg → mp3 → HTTP
	ctx, cancel := context.WithTimeout(r.Context(), 45*time.Second) // Increased to 45s for slower starts
	defer cancel()

	ytdlpCmd := exec.CommandContext(ctx, "yt-dlp",
		"-f", "bestaudio[ext=m4a]/bestaudio", // prefer m4a (faster decode)
		"-o", "-",                            // output to stdout
		"--no-warnings",
		"--quiet",
		vidURL,
	)

	ffmpegCmd := exec.CommandContext(ctx, "ffmpeg",
		"-hide_banner", "-loglevel", "error", // suppress noise
		"-i", "pipe:0",      // input from yt-dlp
		"-f", "mp3",         // output MP3
		"-ac", "2",          // stereo
		"-ar", "44100",      // standard sample rate
		"-b:a", "128k",      // good quality, low bandwidth
		"-vn",               // no video
		"pipe:1",            // output to stdout
	)

	// Connect yt-dlp stdout → ffmpeg stdin
	ytdlpStdout, err := ytdlpCmd.StdoutPipe()
	if err != nil {
		log.Printf("yt-dlp StdoutPipe failed: %v", err)
		http.Error(w, "Internal error", http.StatusInternalServerError)
		return
	}
	ffmpegCmd.Stdin = ytdlpStdout

	// Start yt-dlp
	if err := ytdlpCmd.Start(); err != nil {
		log.Printf("yt-dlp failed to start: %v", err)
		http.Error(w, "Audio fetch failed", http.StatusServiceUnavailable)
		return
	}

	// Get ffmpeg stdout for streaming
	ffmpegStdout, err := ffmpegCmd.StdoutPipe()
	if err != nil {
		ytdlpCmd.Process.Kill()
		log.Printf("ffmpeg StdoutPipe failed: %v", err)
		http.Error(w, "Encoder setup failed", http.StatusInternalServerError)
		return
	}

	// Start ffmpeg
	if err := ffmpegCmd.Start(); err != nil {
		ytdlpCmd.Process.Kill()
		log.Printf("ffmpeg failed to start: %v", err)
		http.Error(w, "Encoding failed", http.StatusInternalServerError)
		return
	}

	// Handle client disconnect
	go func() {
		<-r.Context().Done() // triggered when client disconnects
		ytdlpCmd.Process.Kill()
		ffmpegCmd.Process.Kill()
	}()

	// Stream in chunks
	buf := make([]byte, 64*1024) // 64KB — better for audio streaming
	for {
		n, err := ffmpegStdout.Read(buf)
		if n > 0 {
			if _, writeErr := w.Write(buf[:n]); writeErr != nil {
				// Client gone — clean up
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
	http.HandleFunc("/radio/stream", streamHandler)
	http.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"status":"ok","service":"Radio Backend"}`))
	})

	port := ":8080"
	log.Printf("📻 Radio Server ready at http://localhost%s", port)
	log.Printf("▶️  Try: http://localhost%s/radio/stream?url=https://youtu.be/dQw4w9WgXcQ", port)
	log.Fatal(http.ListenAndServe(port, nil))
}