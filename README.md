# Tama

A terminal-based chat interface for Ollama with vim-style navigation.

## Prerequisites

- [Ollama](https://ollama.ai) running locally on port 11434
- Go 1.25 or later

## Installation

```bash
go build -o tama
```

## Usage

```bash
./tama
```

### Key Bindings

**Prompt Mode:**

- `Enter` — Send message
- `Esc` — Exit to Read Mode

**Read Mode:**

- `i` — Enter Prompt Mode (insert)
- `J` — Next message
- `K` — Previous message
- `G` — Go to bottom of current message
- `gg` — Go to top of current message
- `Ctrl+C` — Cancel ongoing request (or quit if idle)

### Commands

- `clear` — Clear message history
- `exit` or `quit` — Exit the application

## Development

Run tests:

```bash
go test ./...
```

## License

MIT
