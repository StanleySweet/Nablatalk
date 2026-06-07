// Package tts defines the engine interface for text-to-speech synthesis.
package tts

import "context"

// Frame is a single Opus-encoded audio frame or a control signal.
type Frame struct {
	Data       []byte
	Done       bool
	Error      error
	SampleRate int
}

// SynthesisOptions carries per-request parameters that override engine defaults.
type SynthesisOptions struct {
	LengthScale *float64
}

// Engine synthesises text into a stream of Opus audio frames.
type Engine interface {
	Synthesize(ctx context.Context, text, voice string, opts SynthesisOptions) (<-chan Frame, error)
}
