package main

import (
	"encoding/binary"
	"encoding/json"
	"flag"
	"log"
	"net/url"
	"os"
	"time"

	"github.com/gorilla/websocket"
	"gopkg.in/hraban/opus.v2"
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

	dec, err := opus.NewDecoder(24000, 1)
	if err != nil {
		log.Fatalf("decoder: %v", err)
	}

	var allPCM []int16
	var frames int
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
				log.Printf("done — %d frames, %d PCM samples, %s", frames, len(allPCM), elapsed)
				saveWAV("output.wav", allPCM, 24000)
				log.Print("Saved output.wav — play with: afplay output.wav")
				return
			case "error":
				log.Fatalf("error: %s", r.Message)
			}
		}

		frames++
		pcm := make([]int16, 960)
		n, err := dec.Decode(msg, pcm)
		if err != nil {
			log.Fatalf("decode frame %d: %v", frames, err)
		}
		allPCM = append(allPCM, pcm[:n]...)
	}
}

func saveWAV(path string, samples []int16, rate int) {
	f, err := os.Create(path)
	if err != nil {
		log.Fatalf("create: %v", err)
	}
	defer f.Close()

	dataSize := len(samples) * 2

	binary.Write(f, binary.LittleEndian, []byte("RIFF"))
	binary.Write(f, binary.LittleEndian, int32(36+dataSize))
	binary.Write(f, binary.LittleEndian, []byte("WAVE"))
	binary.Write(f, binary.LittleEndian, []byte("fmt "))
	binary.Write(f, binary.LittleEndian, int32(16))       // chunk size
	binary.Write(f, binary.LittleEndian, int16(1))        // PCM
	binary.Write(f, binary.LittleEndian, int16(1))        // mono
	binary.Write(f, binary.LittleEndian, int32(rate))     // sample rate
	binary.Write(f, binary.LittleEndian, int32(rate*2))   // byte rate
	binary.Write(f, binary.LittleEndian, int16(2))        // block align
	binary.Write(f, binary.LittleEndian, int16(16))       // bits per sample
	binary.Write(f, binary.LittleEndian, []byte("data"))
	binary.Write(f, binary.LittleEndian, int32(dataSize))
	binary.Write(f, binary.LittleEndian, samples)
}
