// Package ws implements the WebSocket transport for streaming TTS audio.
package ws

import (
	"encoding/json"
	"log/slog"
	"net/http"

	"github.com/gorilla/websocket"
	"github.com/stan/tts-api/internal/audio"
	"github.com/stan/tts-api/internal/tts"
)

type clientMsg struct {
	Text   string `json:"text"`
	Engine string `json:"engine"`
	Voice  string `json:"voice"`
}

type serverMsg struct {
	Type       string `json:"type"`
	SampleRate int    `json:"sample_rate,omitempty"`
	Message    string `json:"message,omitempty"`
}

var upgrader = websocket.Upgrader{
	CheckOrigin: func(_ *http.Request) bool { return true },
}

// Handler serves the /ws endpoint, accepting one JSON message per connection
// and streaming back Opus audio frames.
type Handler struct {
	engines map[string]tts.Engine
}

// NewHandler creates a WebSocket handler that delegates to the given engines.
func NewHandler(engines map[string]tts.Engine) *Handler {
	return &Handler{engines: engines}
}

// ServeHTTP upgrades to WebSocket, reads a synthesis request, and streams audio.
func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		slog.Warn("ws upgrade", "err", err)
		return
	}
	defer func() { _ = conn.Close() }()

	_, raw, err := conn.ReadMessage()
	if err != nil {
		slog.Warn("ws read", "err", err)
		return
	}

	var req clientMsg
	if err := json.Unmarshal(raw, &req); err != nil {
		writeErr(conn, "invalid json")
		return
	}
	if req.Text == "" {
		writeErr(conn, "text is required")
		return
	}
	if req.Engine == "" {
		req.Engine = "piper"
	}

	engine, ok := h.engines[req.Engine]
	if !ok {
		writeErr(conn, "unknown engine: "+req.Engine)
		return
	}

	frames, err := engine.Synthesize(r.Context(), req.Text, req.Voice)
	if err != nil {
		writeErr(conn, err.Error())
		return
	}

	sampleRate := audio.OpusSampleRate
	first := true

	for frame := range frames {
		if frame.Error != nil {
			writeErr(conn, frame.Error.Error())
			return
		}
		if frame.Done {
			writeJSON(conn, serverMsg{Type: "done"})
			return
		}
		if first {
			if frame.SampleRate > 0 {
				sampleRate = frame.SampleRate
			}
			writeJSON(conn, serverMsg{Type: "start", SampleRate: sampleRate})
			first = false
		}
		if err := conn.WriteMessage(websocket.BinaryMessage, frame.Data); err != nil {
			slog.Warn("ws write", "err", err)
			return
		}
	}
}

func writeErr(conn *websocket.Conn, msg string) {
	if err := conn.WriteJSON(serverMsg{Type: "error", Message: msg}); err != nil {
		slog.Warn("ws write error", "err", err)
	}
}

func writeJSON(conn *websocket.Conn, msg serverMsg) {
	if err := conn.WriteJSON(msg); err != nil {
		slog.Warn("ws write json", "err", err)
	}
}
