// Command server is the TTS API WebSocket server.
package main

import (
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"time"

	"github.com/stan/tts-api/internal/config"
	"github.com/stan/tts-api/internal/tts"
	"github.com/stan/tts-api/internal/ws"
)

func main() {
	cfg := config.Load()

	engines := map[string]tts.Engine{
		"piper": tts.NewPiper(cfg.PiperBin, cfg.PiperModelDir, cfg.PiperSampleRate,
			cfg.PiperLengthScale, cfg.PiperNoiseScale, cfg.PiperNoiseWScale),
	}

	if cfg.ElevenLabsKey != "" {
		engines["elevenlabs"] = tts.NewElevenLabs(cfg.ElevenLabsKey)
		slog.Info("elevenlabs engine enabled")
	}

	mux := http.NewServeMux()
	mux.Handle("/ws", ws.NewHandler(engines))

	srv := &http.Server{
		Addr:              fmt.Sprintf(":%d", cfg.Port),
		Handler:           mux,
		ReadHeaderTimeout: 10 * time.Second,
		ReadTimeout:       30 * time.Second,
		WriteTimeout:      120 * time.Second,
		IdleTimeout:       120 * time.Second,
	}

	slog.Info("listening", "addr", srv.Addr, "engines", len(engines))
	if err := srv.ListenAndServe(); err != nil {
		slog.Error("server", "err", err)
		os.Exit(1)
	}
}
