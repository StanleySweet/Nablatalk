package tts

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os/exec"
	"path/filepath"

	"github.com/stan/tts-api/internal/audio"
)

// Piper runs the piper-tts binary as a subprocess for local synthesis.
type Piper struct {
	bin         string
	modelDir    string
	sampleRate  int
	lengthScale float64
	noiseScale  float64
	noiseWScale float64
}

// NewPiper creates a Piper engine that invokes the given binary.
func NewPiper(bin, modelDir string, sampleRate int, lengthScale, noiseScale, noiseWScale float64) *Piper {
	return &Piper{
		bin:         bin,
		modelDir:    modelDir,
		sampleRate:  sampleRate,
		lengthScale: lengthScale,
		noiseScale:  noiseScale,
		noiseWScale: noiseWScale,
	}
}

// Synthesize runs the piper binary and streams Opus-encoded frames.
func (p *Piper) Synthesize(ctx context.Context, text, voice string) (<-chan Frame, error) {
	ch := make(chan Frame)

	if voice == "" || voice == "default" {
		voice = "fr_FR-upmc-medium"
	}

	modelPath := filepath.Join(p.modelDir, voice+".onnx")

	enc, err := audio.NewEncoder()
	if err != nil {
		return nil, fmt.Errorf("piper: %w", err)
	}

	args := []string{
		"--model", modelPath,
		"--output-raw",
	}
	if p.lengthScale != 1.0 {
		args = append(args, "--length-scale", fmt.Sprintf("%g", p.lengthScale))
	}
	if p.noiseScale != 0.667 {
		args = append(args, "--noise-scale", fmt.Sprintf("%g", p.noiseScale))
	}
	if p.noiseWScale != 0.8 {
		args = append(args, "--noise-w-scale", fmt.Sprintf("%g", p.noiseWScale))
	}

	cmd := exec.CommandContext(ctx, p.bin, args...) //nolint:gosec // binary path from config, text via stdin
	stdin, _ := cmd.StdinPipe()
	stdout, _ := cmd.StdoutPipe()
	stderr, _ := cmd.StderrPipe()

	if err := cmd.Start(); err != nil {
		enc.Close()
		return nil, fmt.Errorf("piper start: %w", err)
	}

	go func() {
		defer close(ch)
		defer enc.Close()

		if _, err := io.WriteString(stdin, text); err != nil {
			slog.Warn("piper stdin write", "err", err)
		}
		if err := stdin.Close(); err != nil {
			slog.Warn("piper stdin close", "err", err)
		}

		go func() {
			if _, err := io.Copy(io.Discard, stderr); err != nil {
				slog.Warn("piper stderr copy", "err", err)
			}
		}()

		samplesPerFrame := p.sampleRate / 50
		buf := make([]byte, 0, samplesPerFrame*2)
		tmp := make([]byte, 4096)
		rd := bufio.NewReader(stdout)

		for {
			n, err := rd.Read(tmp)
			if err != nil && !errors.Is(err, io.EOF) {
				ch <- Frame{Error: fmt.Errorf("piper read: %w", err)}
				return
			}
			buf = append(buf, tmp[:n]...)

			frameBytes := samplesPerFrame * 2
			for len(buf) >= frameBytes {
				pcm := make([]int16, samplesPerFrame)
				for i := range samplesPerFrame {
					pcm[i] = audio.ReadSample(buf[i*2:])
				}
				pcm = audio.Resample(pcm, p.sampleRate)
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
						pcm = audio.Resample(pcm, p.sampleRate)
						frame, err := enc.Encode(pcm)
						if err == nil {
							ch <- Frame{Data: frame, SampleRate: audio.OpusSampleRate}
						}
					}
				}
				break
			}
		}

		if err := cmd.Wait(); err != nil {
			if ctx.Err() != nil {
				return
			}
			slog.Warn("piper wait", "err", err)
		}
		ch <- Frame{Done: true}
	}()

	return ch, nil
}
