package streamer

import (
	"context"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"os/exec"
	"time"
)

// StreamFromURL pipes audio from a YouTube URL directly to an HTTP response writer.
// The stream is MP3, 44.1kHz, stereo, 128kbps — optimized.
func StreamFromURL(ctx context.Context, w http.ResponseWriter, ytURL string) error {
	if !isValidYouTubeURL(ytURL) {
		return fmt.Errorf("invalid YouTube URL: %s", ytURL)
	}

	// Set streaming headers (critical for MTA)
	w.Header().Set("Content-Type", "audio/mpeg")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")

	// Build pipeline: yt-dlp → ffmpeg → MP3 → HTTP
	ytdlpCmd := exec.CommandContext(ctx, "yt-dlp",
		"-f", "bestaudio[ext=m4a]/bestaudio",
		"--no-warnings",
		"--quiet",
		"-o", "-",
		ytURL,
	)

	ffmpegCmd := exec.CommandContext(ctx, "ffmpeg",
		"-hide_banner", "-loglevel", "error",
		"-i", "pipe:0",
		"-f", "mp3",
		"-ac", "2",          // stereo
		"-ar", "44100",      // 44.1kHz
		"-b:a", "128k",      // 128kbps
		"-vn",               // no video
		"pipe:1",
	)

	// Connect pipes
	ytdlpStdout, err := ytdlpCmd.StdoutPipe()
	if err != nil {
		return fmt.Errorf("yt-dlp stdout pipe: %w", err)
	}
	ffmpegCmd.Stdin = ytdlpStdout

	ffmpegStdout, err := ffmpegCmd.StdoutPipe()
	if err != nil {
		return fmt.Errorf("ffmpeg stdout pipe: %w", err)
	}

	// Start processes
	if err := ytdlpCmd.Start(); err != nil {
		return fmt.Errorf("yt-dlp start: %w", err)
	}
	if err := ffmpegCmd.Start(); err != nil {
		ytdlpCmd.Process.Kill()
		return fmt.Errorf("ffmpeg start: %w", err)
	}

	// Ensure cleanup on context cancel (e.g., client disconnect)
	go func() {
		<-ctx.Done()
		ytdlpCmd.Process.Kill()
		ffmpegCmd.Process.Kill()
	}()

	// Stream in 64KB chunks
	buf := make([]byte, 64*1024)
	for {
		n, err := ffmpegStdout.Read(buf)
		if n > 0 {
			if _, writeErr := w.Write(buf[:n]); writeErr != nil {
				// Client gone — cleanup already triggered by ctx.Done()
				return fmt.Errorf("client disconnected")
			}
			if f, ok := w.(http.Flusher); ok {
				f.Flush()
			}
		}
		if err == io.EOF {
			break
		}
		if err != nil {
			return fmt.Errorf("ffmpeg read: %w", err)
		}
	}

	// Wait for processes to exit cleanly
	ffmpegCmd.Wait()
	ytdlpCmd.Wait()

	return nil
}

// isValidYouTubeURL checks if a URL is from YouTube or YouTube Music.
func isValidYouTubeURL(u string) bool {
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

// ServeHTTP is a convenience HTTP handler that extracts ?url= and streams.
// Example: GET /radio/stream?url=https://youtu.be/xyz
func ServeHTTP(w http.ResponseWriter, r *http.Request) {
	ytURL := r.URL.Query().Get("url")
	if ytURL == "" {
		http.Error(w, "Missing ?url=YouTube_URL", http.StatusBadRequest)
		return
	}

	// Use request context (cancels on client disconnect)
	ctx := r.Context()

	// Optional: add timeout for initial fetch (e.g., 45s)
	ctx, cancel := context.WithTimeout(ctx, 45*time.Second)
	defer cancel()

	log.Printf("Streaming: %s", ytURL)
	if err := StreamFromURL(ctx, w, ytURL); err != nil {
		log.Printf("Stream failed: %v", err)
		http.Error(w, "Stream error", http.StatusServiceUnavailable)
	}
}