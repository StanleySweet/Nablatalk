package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"net/url"
	"os"
	"time"

	"github.com/gorilla/websocket"
)

type request struct {
	Text   string `json:"text"`
	Engine string `json:"engine"`
	Voice  string `json:"voice"`
}

type response struct {
	Type       string `json:"type"`
	SampleRate int    `json:"sample_rate,omitempty"`
	Message    string `json:"message,omitempty"`
}

func main() {
	text := flag.String("text", "Bonjour, je suis le lapin Nabaztag.", "text to synthesize")
	voice := flag.String("voice", "fr_FR-upmc-medium", "voice model name")
	engine := flag.String("engine", "piper", "engine: piper or elevenlabs")
	addr := flag.String("addr", "pi4.local:8765", "server address")
	flag.Parse()

	u := url.URL{Scheme: "ws", Host: *addr, Path: "/ws"}
	c, _, err := websocket.DefaultDialer.Dial(u.String(), nil)
	if err != nil {
		log.Fatalf("dial: %v", err)
	}
	defer c.Close()

	req := request{Text: *text, Engine: *engine, Voice: *voice}
	if err := c.WriteJSON(req); err != nil {
		log.Fatalf("write: %v", err)
	}

	out, _ := os.Create("output.opus")
	defer out.Close()

	var frames, bytes int
	start := time.Now()

	for {
		_, msg, err := c.ReadMessage()
		if err != nil {
			log.Fatalf("read: %v", err)
		}

		var r response
		if err := json.Unmarshal(msg, &r); err == nil {
			switch r.Type {
			case "start":
				log.Printf("sample_rate: %d Hz", r.SampleRate)
				continue
			case "done":
				elapsed := time.Since(start)
				log.Printf("done — %d frames, %d bytes, %s", frames, bytes, elapsed)
				fmt.Printf("\nPlay with: ffplay -f opus -ar 24000 -ac 1 output.opus\n")
				return
			case "error":
				log.Fatalf("error: %s", r.Message)
			}
		}

		frames++
		bytes += len(msg)
		out.Write(msg)
	}
}
