package tui

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/glamour"
)

func (m Model) Init() tea.Cmd {
	return tea.Batch(
		checkRunningModel(),
		checkModelStatus(m.CurrentModel),
	)
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd
	var cmd tea.Cmd

	switch msg := msg.(type) {
	case tea.FocusMsg:
		m.Textarea.Focus()
	case tea.BlurMsg:
		m.Textarea.Blur()
	case tea.KeyMsg:
		switch msg.Type {
		case tea.KeyCtrlC:
			// If waiting for a response, cancel it instead of quitting
			if m.IsWaiting || m.ChatRequested {
				m.IsWaiting = false
				m.ChatRequested = false
				if m.cancelCurrentRequestFn != nil {
					m.cancelCurrentRequestFn()
					m.cancelCurrentRequestFn = nil
				}
				// Mark the current message as cancelled
				if len(m.MessagePairs) > 0 && m.CurrentPairIndex < len(m.MessagePairs) {
					m.MessagePairs[m.CurrentPairIndex].Cancelled = true
				}
				return m, nil
			}
			// Otherwise, quit the app
			return m, tea.Quit
		case tea.KeyEsc:
			// Exit prompt mode, enter read mode
			if m.Mode == PromptMode {
				m.Mode = ReadMode
				m.Textarea.Blur()
			}
			m.Viewport.Height = m.calculateViewportHeight()
			return m, nil
		case tea.KeyRunes:
			// Handle 'i' key to enter prompt mode from read mode
			if len(msg.Runes) == 1 && msg.Runes[0] == 'i' && m.Mode == ReadMode {
				// Don't allow entering prompt mode while waiting for a response
				if m.IsWaiting || m.ChatRequested {
					return m, nil
				}
				m.Mode = PromptMode
				m.Textarea.Focus()
				m.LastKeyWasG = false
				m.Viewport.Height = m.calculateViewportHeight()
				return m, nil
			}
			// Handle 'G' key to go to bottom of response
			if len(msg.Runes) == 1 && msg.Runes[0] == 'G' && m.Mode == ReadMode {
				m.Viewport.GotoBottom()
				m.LastKeyWasG = false
				return m, nil
			}
			// Handle 'g' key for 'gg' sequence to go to top
			if len(msg.Runes) == 1 && msg.Runes[0] == 'g' && m.Mode == ReadMode {
				if m.LastKeyWasG {
					// Second 'g' pressed, go to top
					m.Viewport.GotoTop()
					m.LastKeyWasG = false
				} else {
					// First 'g' pressed, wait for second
					m.LastKeyWasG = true
				}
				return m, nil
			}
			// Reset lastKeyWasG for any other key
			if len(msg.Runes) == 1 && m.Mode == ReadMode {
				m.LastKeyWasG = false
			}
			// Handle 'K' key to move to previous message pair
			if len(msg.Runes) == 1 && msg.Runes[0] == 'K' && m.Mode == ReadMode {
				if m.CurrentPairIndex > 0 {
					// Move to previous message pair
					m.CurrentPairIndex--
					m.updateViewport()
					m.Viewport.GotoTop()
				}
				// Do nothing if already at first message
				return m, nil
			}
			// Handle 'J' key to move to next message pair
			if len(msg.Runes) == 1 && msg.Runes[0] == 'J' && m.Mode == ReadMode {
				if m.CurrentPairIndex < len(m.MessagePairs)-1 {
					// Move to next message pair
					m.CurrentPairIndex++
					m.updateViewport()
					m.Viewport.GotoTop()
				}
				// Do nothing if already at last message
				return m, nil
			}
		case tea.KeyEnter:
			if !m.Textarea.Focused() {
				m.Textarea.Focus()
				return m, nil
			}
			// Send message
			input := strings.TrimSpace(m.Textarea.Value())
			if input == "" {
				return m, nil
			}
			// Handle commands
			if input == "clear" {
				m.MessagePairs = []MessagePair{}
				m.CurrentPairIndex = 0
				m.Viewport.SetContent("")
				m.Textarea.Reset()
				return m, nil
			}
			if input == "exit" || input == "quit" {
				return m, tea.Quit
			}

			// Create new message pair with request
			newPair := MessagePair{
				Request:  input,
				Response: "", // Will be filled when response arrives
			}
			m.MessagePairs = append(m.MessagePairs, newPair)
			m.CurrentPairIndex = len(m.MessagePairs) - 1    // Focus on the newly created pair
			m.ResponseTargetIndex = len(m.MessagePairs) - 1 // Response will go to this index

			m.Textarea.Reset()
			m.LoadingModel = true
			m.ModelIsLoaded = false
			m.LoadingStart = time.Now()
			m.RequestStart = time.Now() // Track when request was sent
			m.IsWaiting = false
			m.ChatRequested = true
			m.ResponseLines = []string{}
			m.StreamBuffer = ""

			// Update viewport to show user message immediately
			m.updateViewport()

			m.Mode = ReadMode
			m.Textarea.Blur()
			m.Viewport.Height = m.calculateViewportHeight()
			ctx, cancelFn := context.WithCancel(context.Background())
			m.cancelCurrentRequestFn = cancelFn

			// Save the model being used
			saveLastUsedModel(m.CurrentModel)

			return m, tea.Batch(
				checkModelStatus(m.CurrentModel),
				sendChatRequestCmd(m.MessagePairs, m.CurrentModel, m.Send, ctx, cancelFn, m.ChatURL),
				tickCmd(),
			)
		}

	case tea.WindowSizeMsg:
		atBottom := m.Viewport.AtBottom()
		m.Width = msg.Width
		m.Height = msg.Height

		// Calculate effective width (at least ContentWidth, or window width if smaller)
		effectiveWidth := min(m.Width, ContentWidth)
		viewportHeight := m.calculateViewportHeight()

		if !m.Ready {
			m.Viewport.Width = effectiveWidth
			m.Viewport.Height = viewportHeight
			m.Textarea.SetWidth(effectiveWidth - 4)
			m.Ready = true
			r, _ := glamour.NewTermRenderer(
				glamour.WithStandardStyle("tokyo-night"),
				glamour.WithWordWrap(effectiveWidth),
			)
			m.Renderer = r
		} else {
			m.Viewport.Width = effectiveWidth
			m.Viewport.Height = viewportHeight
			m.Textarea.SetWidth(effectiveWidth - 4)

			if atBottom {
				m.Viewport.GotoBottom()
			}
			r, _ := glamour.NewTermRenderer(
				glamour.WithStandardStyle("tokyo-night"),
				glamour.WithWordWrap(effectiveWidth),
			)
			m.Renderer = r
		}
		m.updateViewport()

	case tickMsg:
		var tickCmds []tea.Cmd
		if m.IsWaiting || m.LoadingModel {
			tickCmds = append(tickCmds, tickCmd())
		}
		if m.LoadingModel {
			tickCmds = append(tickCmds, checkModelStatus(m.CurrentModel))
		}
		if len(tickCmds) > 0 {
			return m, tea.Batch(tickCmds...)
		}

	case ResponseLineMsg:
		m.ResponseLines = []string{}
		m.ResponseLines = append(m.ResponseLines, string(msg))
		m.updateViewport()

	case ResponseCompleteMsg:
		// Response received, stop waiting timer
		m.IsWaiting = false
		m.ChatRequested = false

		response := string(msg)
		response = strings.TrimSpace(response)

		// Calculate response duration and update the target message pair
		if len(m.MessagePairs) > 0 && m.ResponseTargetIndex < len(m.MessagePairs) {
			duration := time.Since(m.RequestStart)
			m.MessagePairs[m.ResponseTargetIndex].Response = response
			m.MessagePairs[m.ResponseTargetIndex].Duration = duration
		}

		// Switch to ReadMode and blur textarea
		m.Mode = ReadMode
		m.Textarea.Blur()

		// Update viewport to show full conversation
		m.Viewport.Height = m.calculateViewportHeight()
		m.updateViewport()

	case modelSelectedMsg:
		m.CurrentModel = msg.model
		saveLastUsedModel(m.CurrentModel)
		// Don't set modelIsLoaded - let modelStatusMsg handle that

	case modelLoadedMsg:
		m.ModelIsLoaded = true
		m.CurrentModel = msg.model
		saveLastUsedModel(m.CurrentModel)

	case modelStatusMsg:
		m.ModelIsLoaded = msg.loaded
		if msg.loaded {
			// Model is now loaded, transition to waiting for response
			m.LoadingModel = false
			if m.ChatRequested {
				m.IsWaiting = true
				m.WaitingStart = time.Now()
			}
		}
		// If not loaded yet, keep polling (tickMsg will continue)

	case SetSendFuncMsg:
		m.Send = msg.Send

	case errorMsg:
		m.Err = msg.err
		m.IsWaiting = false
		m.LoadingModel = false
		return m, nil
	}

	// Only update components based on current mode
	switch m.Mode {
	case PromptMode:
		m.Textarea, cmd = m.Textarea.Update(msg)
		lines := m.Textarea.LineInfo().Height
		m.Textarea.SetHeight(lines)
		//key msg for InputEnd
		inputStartKeyMsg := tea.KeyMsg{Type: tea.KeyCtrlA}
		inputEndKeyMsg := tea.KeyMsg{Type: tea.KeyCtrlE}
		m.Textarea, _ = m.Textarea.Update(inputStartKeyMsg)
		m.Textarea, _ = m.Textarea.Update(inputEndKeyMsg)
		cmds = append(cmds, cmd)

		// Recalculate viewport height when textarea height changes
		m.Viewport.Height = m.calculateViewportHeight()
	case ReadMode:
		m.Viewport, cmd = m.Viewport.Update(msg)
		cmds = append(cmds, cmd)
	}

	return m, tea.Batch(cmds...)
}

// Commands
func tickCmd() tea.Cmd {
	return tea.Tick(100*time.Millisecond, func(t time.Time) tea.Msg {
		return tickMsg(t)
	})
}

func sendChatRequestCmd(messagePairs []MessagePair, modelName string, sendFn func(tea.Msg), ctx context.Context, cancelFn func(), chatURL string) tea.Cmd {
	return func() tea.Msg {
		// Convert message pairs to Ollama messages
		var ollamaMessages []OllamaMessage
		for _, pair := range messagePairs {
			// Skip cancelled messages - they should not be included in context
			if pair.Cancelled {
				continue
			}
			ollamaMessages = append(ollamaMessages, OllamaMessage{
				Role:    "user",
				Content: pair.Request,
			})
			if pair.Response != "" {
				ollamaMessages = append(ollamaMessages, OllamaMessage{
					Role:    "assistant",
					Content: pair.Response,
				})
			}
		}

		reqBody := ChatRequest{
			Model:    modelName,
			Messages: ollamaMessages,
			Stream:   true,
		}

		jsonData, err := json.Marshal(reqBody)
		if err != nil {
			return errorMsg{err: fmt.Errorf("failed to marshal request: %w", err)}
		}

		req, err := http.NewRequestWithContext(ctx, "POST", chatURL, bytes.NewBuffer(jsonData))
		req.Header.Set("Content-Type", "application/json")
		client := &http.Client{}

		resp, err := client.Do(req)

		if err != nil {
			return errorMsg{err: fmt.Errorf("failed to send request: %w", err)}
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			body, _ := io.ReadAll(resp.Body)
			return errorMsg{err: fmt.Errorf("request failed with status %d: %s", resp.StatusCode, string(body))}
		}

		// Stream the response
		scanner := bufio.NewScanner(resp.Body)
		var fullResponse strings.Builder
		outChan := make(chan []byte)

		go func() {
			for scanner.Scan() {
				outChan <- scanner.Bytes()
			}
			cancelFn()
		}()

		for {
			select {
			case <-ctx.Done():
				return ResponseCompleteMsg(fullResponse.String())
			case responseBytes := <-outChan:
				var streamResp StreamResponse
				copiedResponseBytes := make([]byte, len(responseBytes))
				copy(copiedResponseBytes, responseBytes)
				if err := json.Unmarshal(copiedResponseBytes, &streamResp); err != nil {
					return ResponseCompleteMsg(fullResponse.String())
				}

				if streamResp.Message.Content != "" {
					fullResponse.WriteString(streamResp.Message.Content)
				}
				// Send partial updates for streaming effect
				sendFn(ResponseLineMsg(fullResponse.String()))
			}
		}
	}
}

func checkRunningModel() tea.Cmd {
	return func() tea.Msg {
		resp, err := http.Get(ollamaModelsURL)
		if err != nil {
			return errorMsg{err: err}
		}
		defer resp.Body.Close()

		var modelsResp ModelsResponse
		if err := json.NewDecoder(resp.Body).Decode(&modelsResp); err != nil {
			return errorMsg{err: err}
		}

		if len(modelsResp.Models) > 0 {
			return modelLoadedMsg{model: modelsResp.Models[0].Name}
		}

		// No models running, use last used model
		lastModel := loadLastUsedModel()
		if lastModel != "" {
			return modelSelectedMsg{model: lastModel}
		}

		return modelSelectedMsg{model: defaultModel}
	}
}

func checkModelStatus(modelName string) tea.Cmd {
	return func() tea.Msg {
		resp, err := http.Get(ollamaModelsURL)
		if err != nil {
			return errorMsg{err: err}
		}
		defer resp.Body.Close()

		var modelsResp ModelsResponse
		if err := json.NewDecoder(resp.Body).Decode(&modelsResp); err != nil {
			return errorMsg{err: err}
		}

		// Check if the specific model is loaded
		for _, m := range modelsResp.Models {
			if m.Name == modelName {
				return modelStatusMsg{loaded: true}
			}
		}

		return modelStatusMsg{loaded: false}
	}
}
