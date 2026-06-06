// Package config loads the server configuration from environment variables.
package config

import (
	"os"
	"strconv"
)

// Config holds all server configuration.
type Config struct {
	Port             int
	PiperBin         string
	PiperModelDir    string
	PiperSampleRate  int
	PiperLengthScale float64
	PiperNoiseScale  float64
	PiperNoiseWScale float64
	ElevenLabsKey    string
	DefaultEngine    string
	DefaultVoice     string
}

// Load reads configuration from environment variables, applying defaults.
func Load() *Config {
	return &Config{
		Port:             envInt("TTS_PORT", 8765),
		PiperBin:         envStr("TTS_PIPER_BIN", "/opt/piper/piper"),
		PiperModelDir:    envStr("TTS_PIPER_MODEL_DIR", "/models"),
		PiperSampleRate:  envInt("TTS_PIPER_SAMPLE_RATE", 22050),
		PiperLengthScale: envFloat("TTS_PIPER_LENGTH_SCALE", 1.3),
		PiperNoiseScale:  envFloat("TTS_PIPER_NOISE_SCALE", 0.667),
		PiperNoiseWScale: envFloat("TTS_PIPER_NOISE_W_SCALE", 0.8),
		ElevenLabsKey:    envStr("TTS_ELEVENLABS_KEY", ""),
		DefaultEngine:    envStr("TTS_DEFAULT_ENGINE", "piper"),
		DefaultVoice:     envStr("TTS_DEFAULT_VOICE", "fr_FR-upmc-medium"),
	}
}

func envStr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func envInt(key string, fallback int) int {
	if v := os.Getenv(key); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			return n
		}
	}
	return fallback
}

func envFloat(key string, fallback float64) float64 {
	if v := os.Getenv(key); v != "" {
		if f, err := strconv.ParseFloat(v, 64); err == nil {
			return f
		}
	}
	return fallback
}
