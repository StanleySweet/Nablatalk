package tts

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"

	"github.com/hajimehoshi/go-mp3"
	"github.com/stan/tts-api/internal/audio"
)

// ElevenLabs uses the ElevenLabs Text-to-Speech API for cloud-based synthesis.
type ElevenLabs struct {
	apiKey string
}

// NewElevenLabs creates an ElevenLabs engine with the given API key.
func NewElevenLabs(apiKey string) *ElevenLabs {
	return &ElevenLabs{apiKey: apiKey}
}

// Synthesize calls the ElevenLabs TTS API and streams Opus-encoded frames.
func (e *ElevenLabs) Synthesize(ctx context.Context, text, voice string) (<-chan Frame, error) {
	ch := make(chan Frame)

	if voice == "" || voice == "default" {
		voice = "21m00Tcm4TlvDq8ikWAM"
	}

	go func() {
		defer close(ch)

		payload := fmt.Sprintf(
			`{"text":%q,"model_id":"eleven_monolingual_v1","voice_settings":{"stability":0.5,"similarity_boost":0.75}}`,
			text,
		)

		req, err := http.NewRequestWithContext(ctx, http.MethodPost,
			"https://api.elevenlabs.io/v1/text-to-speech/"+voice,
			bytes.NewReader([]byte(payload)),
		)
		if err != nil {
			ch <- Frame{Error: fmt.Errorf("elevenlabs req: %w", err)}
			return
		}
		req.Header.Set("Xi-Api-Key", e.apiKey)
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Accept", "audio/mpeg")

		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			ch <- Frame{Error: fmt.Errorf("elevenlabs http: %w", err)}
			return
		}
		defer func() { _ = resp.Body.Close() }()

		if resp.StatusCode != http.StatusOK {
			body, _ := io.ReadAll(resp.Body)
			ch <- Frame{Error: fmt.Errorf("elevenlabs status %d: %s", resp.StatusCode, string(body))}
			return
		}

		mp3Data, err := io.ReadAll(resp.Body)
		if err != nil {
			ch <- Frame{Error: fmt.Errorf("elevenlabs read: %w", err)}
			return
		}

		decoder, err := mp3.NewDecoder(bytes.NewReader(mp3Data))
		if err != nil {
			ch <- Frame{Error: fmt.Errorf("elevenlabs mp3: %w", err)}
			return
		}

		sr := decoder.SampleRate()

		enc, err := audio.NewEncoder()
		if err != nil {
			ch <- Frame{Error: err}
			return
		}
		defer enc.Close()

		// Input frame = 20ms at the MP3 decoder's sample rate
		samplesPerFrame := sr / 50
		buf := make([]byte, 0, samplesPerFrame*2)
		tmp := make([]byte, 4096)

		for {
			n, err := decoder.Read(tmp)
			if err != nil && !errors.Is(err, io.EOF) {
				ch <- Frame{Error: fmt.Errorf("elevenlabs decode: %w", err)}
				return
			}
			buf = append(buf, tmp[:n]...)

			frameBytes := samplesPerFrame * 2
			for len(buf) >= frameBytes {
				pcm := make([]int16, samplesPerFrame)
				for i := range samplesPerFrame {
					pcm[i] = audio.ReadSample(buf[i*2:])
				}
				pcm = audio.Resample(pcm, sr)
				frame, err := enc.Encode(pcm)
				if err != nil {
					ch <- Frame{Error: err}
					return
				}
				ch <- Frame{Data: frame, SampleRate: audio.OpusSampleRate}
				buf = buf[frameBytes:]
			}

			if errors.Is(err, io.EOF) {
				if len(buf) > 0 {
					rem := len(buf) / 2
					pcm := make([]int16, rem)
					for i := range rem {
						pcm[i] = audio.ReadSample(buf[i*2:])
					}
					if len(pcm) >= 48 {
						pcm = audio.Resample(pcm, sr)
						frame, err := enc.Encode(pcm)
						if err == nil {
							ch <- Frame{Data: frame, SampleRate: audio.OpusSampleRate}
						}
					}
				}
				break
			}
		}

		ch <- Frame{Done: true}
	}()

	return ch, nil
}
