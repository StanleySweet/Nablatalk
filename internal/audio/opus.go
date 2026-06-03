// Package audio provides Opus audio encoding for streaming TTS output.
package audio

import (
	"encoding/binary"
	"fmt"
	"time"

	"gopkg.in/hraban/opus.v2"
)

// OpusSampleRate is the fixed Opus encoding rate (the only valid rates are
// 8000, 12000, 16000, 24000, 48000). We use 24000 for good speech quality
// and convenient 20ms frame alignment (480 samples).
const OpusSampleRate = 24000

// FrameDuration is the duration of each Opus audio frame.
const FrameDuration = 20 * time.Millisecond

// OpusFrameSamples is the number of PCM samples per Opus frame at OpusSampleRate.
const OpusFrameSamples = int(FrameDuration / (time.Second / OpusSampleRate))

// ReadSample decodes a 16-bit little-endian PCM sample from b.
// b must have at least 2 bytes.
func ReadSample(b []byte) int16 {
	return int16(binary.LittleEndian.Uint16(b)) //nolint:gosec // intentional PCM sample conversion
}

// Encoder wraps an Opus encoder configured for mono speech.
type Encoder struct {
	enc       *opus.Encoder
	frameSize int
}

// NewEncoder creates an Opus encoder for mono speech at the fixed OpusSampleRate.
func NewEncoder() (*Encoder, error) {
	enc, err := opus.NewEncoder(OpusSampleRate, 1, opus.AppVoIP)
	if err != nil {
		return nil, fmt.Errorf("opus encoder: %w", err)
	}
	return &Encoder{
		enc:       enc,
		frameSize: OpusFrameSamples,
	}, nil
}

// FrameSize returns the number of PCM samples per Opus frame (always OpusFrameSamples).
func (e *Encoder) FrameSize() int { return e.frameSize }

// Encode encodes 16-bit mono PCM samples into an Opus frame.
// The caller must ensure len(pcm) == FrameSize().
func (e *Encoder) Encode(pcm []int16) ([]byte, error) {
	out := make([]byte, 1500)
	n, err := e.enc.Encode(pcm, out)
	if err != nil {
		return nil, fmt.Errorf("opus encode: %w", err)
	}
	return out[:n], nil
}

// Close is a no-op kept for interface consistency.
func (e *Encoder) Close() {}

// Resample converts PCM from inRate to OpusSampleRate using linear interpolation.
func Resample(in []int16, inRate int) []int16 {
	if inRate == OpusSampleRate || len(in) == 0 {
		return in
	}
	ratio := float64(inRate) / float64(OpusSampleRate)
	outLen := int(float64(len(in)) / ratio)
	out := make([]int16, outLen)
	for i := range out {
		src := float64(i) * ratio
		idx := int(src)
		frac := src - float64(idx)
		left := in[idx]
		right := left
		if idx+1 < len(in) {
			right = in[idx+1]
		}
		out[i] = int16(float64(left)*(1-frac) + float64(right)*frac)
	}
	return out
}
