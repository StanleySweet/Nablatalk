# TTS API

Lightweight WebSocket TTS server for Raspberry Pi 4. Piper TTS (local) + optional ElevenLabs API.

## Architecture

```
pynab (Pi Zero) ──WebSocket──► tts-api (Pi 4, Docker)
                                  ├── Piper TTS (subprocess) → PCM → Opus
                                  └── ElevenLabs (HTTP API)  → MP3 → PCM → Opus
```

Go server, single static binary. Each engine runs in a goroutine, streams Opus frames (20ms, AppVoIP) back over the WebSocket.

## Protocol

Connect via WebSocket to `/ws`, send one JSON message:

```json
{"text": "Hello world", "engine": "piper", "voice": "default"}
```

Server responds with:
- `{"type": "start", "sample_rate": 24000}` — metadata
- Binary messages — Opus audio frames (20ms each)
- `{"type": "done"}` — synthesis complete
- `{"type": "error", "message": "..."}` — error

## Environment variables

| Variable | Default | Description |
|---|---|---|
| `TTS_PORT` | `8765` | WebSocket listen port |
| `TTS_PIPER_BIN` | `/usr/local/bin/piper` | Piper binary path |
| `TTS_PIPER_MODEL_DIR` | `/models` | Directory containing `.onnx` voice models |
| `TTS_PIPER_SAMPLE_RATE` | `22050` | Piper model sample rate |
| `TTS_PIPER_LENGTH_SCALE` | `1.0` | Speed (>1 = slower, <1 = faster) |
| `TTS_PIPER_NOISE_SCALE` | `0.667` | Voice variation (0 = robotic, 1 = natural) |
| `TTS_PIPER_NOISE_W_SCALE` | `0.8` | Prosody variation (0 = monotone) |
| `TTS_ELEVENLABS_KEY` | — | ElevenLabs API key (omit to disable) |

### Voices

Set `voice` in the request to select a model from `TTS_PIPER_MODEL_DIR`. For example, with `voice: "fr_FR-upmc-medium"` the server loads `/models/fr_FR-upmc-medium.onnx`.

Download voices from [rhasspy/piper-voices](https://huggingface.co/rhasspy/piper-voices). Popular ones:

| Voice | Language | Request value |
|---|---|---|
| en_US-lessac-medium | English (US) | `en_US-lessac-medium` |
| en_GB-semilow-medium | English (UK) | `en_GB-semilow-medium` |
| fr_FR-upmc-medium | French | `fr_FR-upmc-medium` |

## Install on Raspberry Pi 4

```bash
# 1. Clone the repo on your Pi 4
git clone <your-repo-url> /home/pi/tts-api
cd /home/pi/tts-api

# 2. Download a voice model
mkdir -p models
# French:
curl -fsSLo models/fr_FR-upmc-medium.onnx \
  "https://huggingface.co/rhasspy/piper-voices/resolve/v1.0.0/fr/fr_FR/upmc/medium/fr_FR-upmc-medium.onnx?download=true"
curl -fsSLo models/fr_FR-upmc-medium.onnx.json \
  "https://huggingface.co/rhasspy/piper-voices/resolve/v1.0.0/fr/fr_FR/upmc/medium/fr_FR-upmc-medium.onnx.json?download=true"

# English (optional):
curl -fsSLo models/en_US-lessac-medium.onnx \
  "https://huggingface.co/rhasspy/piper-voices/resolve/v1.0.0/en/en_US/lessac/medium/en_US-lessac-medium.onnx?download=true"

# 3. Build and run (takes ~2-3 min on Pi 4)
docker compose up -d

# 4. Check it's running
docker compose logs -f

# 5. (Optional) Enable ElevenLabs
echo "TTS_ELEVENLABS_KEY=your_key" >> .env
docker compose up -d
```

### From Pi Zero (pynab)

```python
import asyncio, json
import websockets

async def say(text, voice="fr_FR-upmc-medium"):
    async with websockets.connect("ws://pi4.local:8765/ws") as ws:
        await ws.send(json.dumps({"text": text, "engine": "piper", "voice": voice}))
        meta = json.loads(await ws.recv())
        while True:
            msg = await ws.recv()
            if isinstance(msg, bytes):
                # opus frame — send to rabbit or decode locally
                pass
            else:
                ctrl = json.loads(msg)
                if ctrl["type"] == "done":
                    break
                if ctrl["type"] == "error":
                    raise Exception(ctrl["message"])
```

### Tuning the voice (Nabaztag-like)

For a more robotic/synthetic sound closer to the original Nabaztag, set in `docker-compose.yml`:

```yaml
environment:
  - TTS_PIPER_NOISE_SCALE=0.3      # lower = more robotic
  - TTS_PIPER_NOISE_W_SCALE=0.4    # lower = more monotone
```

### Local dev (Mac)

```bash
brew install libopus opusfile
go run ./cmd/server/
```

## Project structure

```
├── cmd/server/main.go              # Entry point, HTTP server with timeouts
├── internal/
│   ├── audio/opus.go               # Opus encoder (20ms frames, AppVoIP)
│   ├── config/config.go            # Env-var config with defaults
│   ├── tts/
│   │   ├── interface.go            # Engine interface + Frame type
│   │   ├── piper.go                # Piper subprocess → PCM → Opus
│   │   └── elevenlabs.go           # ElevenLabs API → MP3 → PCM → Opus
│   └── ws/handler.go               # WebSocket handler, streaming protocol
├── Dockerfile                      # Multi-stage: Go build + Piper binary
├── docker-compose.yml
├── .golangci.yml                   # Lint config (v2, default:all)
├── Makefile                        # build / lint / vet / fmt / clean
└── .env.example
```

## Development

```bash
make lint       # golangci-lint run
make vet        # go vet
make fmt        # gofmt -s -w
make build      # CGO_ENABLED=1 go build -o bin/tts-api
```

Logging uses `log/slog` (Go 1.21+ stdlib) — structured key-value output.
