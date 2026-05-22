# aloof-agentic-repl
An agentic bench tool for testing ideas in REPL fashion.

## Requirements

- Go 1.21+
- An Ollama-compatible AI endpoint

## Configuration

All configuration is via environment variables:

| Variable | Description | Example |
|---|---|---|
| `ALOOF_API_KEY` | Bearer token for the API endpoint | `e3b0c44298fc1c149a...` |
| `ALOOF_ENDPOINT_KEY` | Full URL of the `/api/generate` endpoint | `https://ai.example.com/api/generate` |
| `ALOOF_ENDPOINT_MODEL` | Model name to use | `deepseek-r1:1.5b` |
| `ALOOF_DEBUG` | Set to `1` to print raw chunk data to stderr | `1` |

## Usage

```sh
export ALOOF_API_KEY=your_api_key
export ALOOF_ENDPOINT_KEY=https://your-endpoint/api/generate
export ALOOF_ENDPOINT_MODEL=deepseek-r1:1.5b

go run .
```

Type a prompt and press Enter. The response streams in real time. Use Ctrl+D or Ctrl+C to exit.

## Features

- **Streaming responses** — tokens are printed as they arrive.
- **Thinking display** — `<think>…</think>` blocks (emitted by reasoning models like DeepSeek-R1) are rendered in a distinct dim/italic style so they are visually separated from the final answer.
- **Conversation memory** — the context token array returned by the endpoint is stored and sent back with each subsequent prompt, preserving conversation history for the duration of the session.
