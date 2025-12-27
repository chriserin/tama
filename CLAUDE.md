# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Commands

```bash
go build -o tama                                    # Build
go test ./...                                       # All tests
go test -v ./internal/tui -run TestName            # Single test
./tama                                              # Run (requires Ollama on localhost:11434)
```

## Architecture

Bubble Tea MVU pattern. Code split into:
- `main.go`: 29-line entry point, sends `SetSendFuncMsg` to initialize
- `internal/tui/model.go`: Types, Model struct, initialization
- `internal/tui/update.go`: All state transitions, message handlers
- `internal/tui/view.go`: Pure rendering (renders only current message pair)

## Critical Patterns

**Response Targeting**: Responses go to `ResponseTargetIndex`, NOT `CurrentPairIndex`. This allows navigating between messages while a response streams to a different message.

When Enter pressed:
1. New MessagePair created, both indices set to new pair
2. User can navigate (`J`/`K`) changing `CurrentPairIndex`
3. Streaming responses still update `MessagePairs[ResponseTargetIndex]`
4. Partial: `ResponseLineMsg` → `ResponseLines[]` (shown only when viewing ResponseTargetIndex)
5. Complete: `ResponseCompleteMsg` → `MessagePairs[ResponseTargetIndex].Response`

**Request Cancellation**: `Ctrl+C` cancels if `IsWaiting`, else quits. Cancelled messages excluded from future context.

**Mode Transitions**: Cannot enter PromptMode while `IsWaiting || ChatRequested`. Shows "Waiting for response" message.

## Testing

- Follow TDD: write test first, verify failure, implement, verify pass
- Check `features/*.feat` for scenario requirements
- Use `httptest` for HTTP: `ChatURL` field configurable, pass mock server URL to `sendChatRequestCmd`
- Don't assert view strings (glamour adds ANSI codes), test model data directly
- Navigation tests: `J`/`K` always reset `YOffset = 0`

## Feature Implementation

1. Read `features/*.feat` scenario
2. Write failing test (BDD-style naming: `TestCannotEnterPromptMode...`)
3. Implement in model/update/view
4. Verify tests pass
5. Run full suite
