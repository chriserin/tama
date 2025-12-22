package main

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/charmbracelet/bubbles/cursor"
	"github.com/charmbracelet/bubbles/textarea"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/glamour"
	"github.com/charmbracelet/lipgloss"
)

// Ollama API types
type OllamaMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type ChatRequest struct {
	Model    string          `json:"model"`
	Messages []OllamaMessage `json:"messages"`
	Stream   bool            `json:"stream"`
}

type ChatResponse struct {
	Model     string        `json:"model"`
	CreatedAt string        `json:"created_at"`
	Message   OllamaMessage `json:"message"`
	Done      bool          `json:"done"`
}

type StreamResponse struct {
	Model     string        `json:"model"`
	CreatedAt string        `json:"created_at"`
	Message   OllamaMessage `json:"message"`
	Done      bool          `json:"done"`
}

// Application types
type MessagePair struct {
	Request  string
	Response string
	Duration time.Duration // Time taken to generate the response
}

type ModelsResponse struct {
	Models []struct {
		Name string `json:"name"`
	} `json:"models"`
}

const (
	ollamaChatURL   = "http://localhost:11434/api/chat"
	ollamaModelsURL = "http://localhost:11434/api/ps"
	defaultModel    = "gpt-oss:20b"
	contentWidth    = 100
)

// Mode represents the current interaction mode
type Mode int

const (
	PromptMode Mode = iota
	ReadMode
)

// Bubbletea messages
type tickMsg time.Time
type responseLineMsg string
type errorMsg struct{ err error }
type modelLoadedMsg struct{ model string }
type modelSelectedMsg struct{ model string }
type modelStatusMsg struct{ loaded bool }

// Model holds the application state
type model struct {
	mode             Mode
	textarea         textarea.Model
	viewport         viewport.Model
	messagePairs     []MessagePair
	currentPairIndex int // 0-based index of currently focused message pair
	currentModel     string
	modelIsLoaded    bool
	err              error
	width            int
	height           int
	ready            bool
	renderer         *glamour.TermRenderer
	loadingStart     time.Time
	waitingStart     time.Time
	requestStart     time.Time // Time when current request was sent
	isWaiting        bool
	chatRequested    bool
	loadingModel     bool
	responseLines    []string
	streamBuffer     string
}

func initialModel() model {
	ta := textarea.New()
	ta.Placeholder = "Type your message..."
	ta.Focus()
	ta.CharLimit = 0
	ta.SetWidth(contentWidth - 4)
	ta.SetHeight(1)
	ta.ShowLineNumbers = false
	ta.FocusedStyle.CursorLine = lipgloss.NewStyle()
	ta.Cursor.SetMode(cursor.CursorStatic)

	vp := viewport.New(contentWidth, 20)

	// Initialize markdown renderer
	r, _ := glamour.NewTermRenderer(
		glamour.WithStandardStyle("tokyo-night"),
		glamour.WithWordWrap(contentWidth),
	)

	return model{
		mode:             PromptMode,
		textarea:         ta,
		viewport:         vp,
		messagePairs:     []MessagePair{},
		currentPairIndex: 0,
		currentModel:     loadLastUsedModel(),
		renderer:         r,
	}
}

func (m model) Init() tea.Cmd {
	return tea.Batch(
		checkRunningModel(),
		checkModelStatus(m.currentModel),
	)
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd
	var cmd tea.Cmd

	switch msg := msg.(type) {
	case tea.FocusMsg:
		m.textarea.Focus()
	case tea.BlurMsg:
		m.textarea.Blur()
	case tea.KeyMsg:
		switch msg.Type {
		case tea.KeyCtrlC:
			return m, tea.Quit
		case tea.KeyEsc:
			// Exit prompt mode, enter read mode
			if m.mode == PromptMode {
				m.mode = ReadMode
				m.textarea.Blur()
				return m, nil
			}
			return m, nil
		case tea.KeyRunes:
			// Handle 'i' key to enter prompt mode from read mode
			if len(msg.Runes) == 1 && msg.Runes[0] == 'i' && m.mode == ReadMode {
				m.mode = PromptMode
				m.textarea.Focus()
				return m, nil
			}
			// Handle 'K' key to scroll to top or move to previous message pair
			if len(msg.Runes) == 1 && msg.Runes[0] == 'K' && m.mode == ReadMode {
				if m.viewport.AtTop() && m.currentPairIndex > 0 {
					// At top, move to previous message pair
					m.currentPairIndex--
					m.updateViewport()
					m.viewport.GotoTop()
				} else {
					// Not at top, scroll to top
					m.viewport.GotoTop()
				}
				return m, nil
			}
			// Handle 'J' key to scroll to bottom or move to next message pair
			if len(msg.Runes) == 1 && msg.Runes[0] == 'J' && m.mode == ReadMode {
				if m.viewport.AtBottom() && m.currentPairIndex < len(m.messagePairs)-1 {
					// At bottom, move to next message pair
					m.currentPairIndex++
					m.updateViewport()
					m.viewport.GotoTop()
				} else {
					// Not at bottom, scroll to bottom
					m.viewport.GotoBottom()
				}
				return m, nil
			}
		case tea.KeyEnter:
			if !m.textarea.Focused() {
				m.textarea.Focus()
				return m, nil
			}
			// Send message
			input := strings.TrimSpace(m.textarea.Value())
			if input == "" {
				return m, nil
			}
			// Handle commands
			if input == "clear" {
				m.messagePairs = []MessagePair{}
				m.currentPairIndex = 0
				m.viewport.SetContent("")
				m.textarea.Reset()
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
			m.messagePairs = append(m.messagePairs, newPair)
			m.currentPairIndex = len(m.messagePairs) - 1 // Focus on the newly created pair

			m.textarea.Reset()
			m.loadingModel = true
			m.modelIsLoaded = false
			m.loadingStart = time.Now()
			m.requestStart = time.Now() // Track when request was sent
			m.isWaiting = false
			m.chatRequested = true
			m.responseLines = []string{}
			m.streamBuffer = ""

			// Update viewport to show user message immediately
			m.updateViewport()

			m.mode = ReadMode
			m.textarea.Blur()
			m.viewport.Height = m.calculateViewportHeight()

			// Save the model being used
			saveLastUsedModel(m.currentModel)

			return m, tea.Batch(
				checkModelStatus(m.currentModel),
				sendChatRequestCmd(m.messagePairs, m.currentModel),
				tickCmd(),
			)
		}

	case tea.WindowSizeMsg:
		atBottom := m.viewport.AtBottom()
		m.width = msg.Width
		m.height = msg.Height

		// Calculate effective width (at least contentWidth, or window width if smaller)
		effectiveWidth := min(m.width, contentWidth)
		viewportHeight := m.calculateViewportHeight()

		if !m.ready {
			m.viewport = viewport.New(effectiveWidth, viewportHeight)
			m.textarea.SetWidth(effectiveWidth - 4)
			m.ready = true
			r, _ := glamour.NewTermRenderer(
				glamour.WithStandardStyle("tokyo-night"),
				glamour.WithWordWrap(effectiveWidth),
			)
			m.renderer = r
		} else {
			m.viewport.Width = effectiveWidth
			m.viewport.Height = viewportHeight
			m.textarea.SetWidth(effectiveWidth - 4)

			if atBottom {
				m.viewport.GotoBottom()
			}
			r, _ := glamour.NewTermRenderer(
				glamour.WithStandardStyle("tokyo-night"),
				glamour.WithWordWrap(effectiveWidth),
			)
			m.renderer = r
		}
		m.updateViewport()

	case tickMsg:
		var tickCmds []tea.Cmd
		if m.isWaiting || m.loadingModel {
			tickCmds = append(tickCmds, tickCmd())
		}
		if m.loadingModel {
			tickCmds = append(tickCmds, checkModelStatus(m.currentModel))
		}
		if len(tickCmds) > 0 {
			return m, tea.Batch(tickCmds...)
		}

	case responseLineMsg:
		// Response received, stop waiting timer
		m.isWaiting = false
		m.chatRequested = false

		response := string(msg)
		response = strings.TrimSpace(response)

		// Calculate response duration and update the current message pair
		if len(m.messagePairs) > 0 {
			duration := time.Since(m.requestStart)
			m.messagePairs[m.currentPairIndex].Response = response
			m.messagePairs[m.currentPairIndex].Duration = duration
		}

		// Clear streaming buffer and response lines
		m.streamBuffer = ""
		m.responseLines = []string{}

		// Switch to ReadMode and blur textarea
		m.mode = ReadMode
		m.textarea.Blur()

		// Update viewport to show full conversation
		m.updateViewport()

	case modelSelectedMsg:
		m.currentModel = msg.model
		saveLastUsedModel(m.currentModel)
		// Don't set modelIsLoaded - let modelStatusMsg handle that

	case modelLoadedMsg:
		m.modelIsLoaded = true
		m.currentModel = msg.model
		saveLastUsedModel(m.currentModel)

	case modelStatusMsg:
		m.modelIsLoaded = msg.loaded
		if msg.loaded {
			// Model is now loaded, transition to waiting for response
			m.loadingModel = false
			if m.chatRequested {
				m.isWaiting = true
				m.waitingStart = time.Now()
			}
		}
		// If not loaded yet, keep polling (tickMsg will continue)

	case errorMsg:
		m.err = msg.err
		m.isWaiting = false
		m.loadingModel = false
		return m, nil
	}

	// Only update components based on current mode
	switch m.mode {
	case PromptMode:
		m.textarea, cmd = m.textarea.Update(msg)
		lines := m.textarea.LineInfo().Height
		m.textarea.SetHeight(lines)
		//key msg for InputEnd
		inputStartKeyMsg := tea.KeyMsg{Type: tea.KeyCtrlA}
		inputEndKeyMsg := tea.KeyMsg{Type: tea.KeyCtrlE}
		m.textarea, _ = m.textarea.Update(inputStartKeyMsg)
		m.textarea, _ = m.textarea.Update(inputEndKeyMsg)
		cmds = append(cmds, cmd)

		// Recalculate viewport height when textarea height changes
		m.viewport.Height = m.calculateViewportHeight()
	case ReadMode:
		m.viewport, cmd = m.viewport.Update(msg)
		cmds = append(cmds, cmd)
	}

	return m, tea.Batch(cmds...)
}

func (m *model) calculateViewportHeight() int {
	// Layout: TAMA line (1) + viewport + blank (1) + input borders (2) + textarea (dynamic) + status (1)
	fixedHeight := 3
	textareaHeight := m.textarea.Height()
	inputBorders := 2
	if m.mode == ReadMode {
		textareaHeight = 0
		inputBorders = 0
	}
	return max(m.height-fixedHeight-textareaHeight-inputBorders, 5)
}

func (m *model) updateViewport() {
	var content strings.Builder

	// Display only the current message pair
	if len(m.messagePairs) > 0 && m.currentPairIndex < len(m.messagePairs) {
		pair := m.messagePairs[m.currentPairIndex]

		// Request message with border (straight line)
		requestBorderText := "──── Request "
		remainingWidth := max(m.viewport.Width-utf8.RuneCountInString(requestBorderText), 0)
		requestBorder := lipgloss.NewStyle().
			Foreground(lipgloss.Color("240")).
			Render(requestBorderText + strings.Repeat("─", remainingWidth))

		content.WriteString(requestBorder)
		content.WriteString("\n")
		content.WriteString(pair.Request)
		content.WriteString("\n\n")

		// Response message (if present)
		if pair.Response != "" {
			// Response border with duration (straight line)
			durationStr := fmt.Sprintf("%.1fs", pair.Duration.Seconds())
			responseBorderText := fmt.Sprintf("──── Response (%s) ", durationStr)
			remainingWidth := max(m.viewport.Width-utf8.RuneCountInString(responseBorderText), 0)
			responseBorder := lipgloss.NewStyle().
				Foreground(lipgloss.Color("240")).
				Render(responseBorderText + strings.Repeat("─", remainingWidth))

			content.WriteString(responseBorder)
			content.WriteString("\n")

			// Render response as markdown
			rendered, err := m.renderer.Render(pair.Response)
			if err != nil {
				content.WriteString(pair.Response)
			} else {
				content.WriteString(rendered)
			}
			content.WriteString("\n")
		}
	}

	// If currently streaming, show partial response
	if len(m.responseLines) > 0 {
		content.WriteString(lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("13")).
			Render("Assistant:"))
		content.WriteString("\n")
		content.WriteString(strings.Join(m.responseLines, ""))
	}

	m.viewport.SetContent(content.String())
	m.viewport.GotoTop()
}

func (m model) View() string {
	if !m.ready {
		return "Initializing..."
	}

	var b strings.Builder

	// Calculate effective width (at least contentWidth, or window width if smaller)
	effectiveWidth := min(m.width, contentWidth)

	// Calculate left padding to center the content block
	leftPadding := max((m.width-effectiveWidth)/2, 0)

	// Style for positioning content in the center with left padding
	contentStyle := lipgloss.NewStyle().
		PaddingLeft(leftPadding)

	// Top: TAMA header with horizontal line on same line (centered)
	tamaText := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("205")).
		Render("TAMA")

	// Calculate remaining width for horizontal line (accounting for "TAMA " with space)
	tamaWidth := 5 // "TAMA " = 4 chars + 1 space
	lineWidth := max(effectiveWidth-tamaWidth, 0)
	horizontalLine := strings.Repeat("─", lineWidth)

	topLine := tamaText + " " + horizontalLine
	b.WriteString(contentStyle.Render(topLine))
	b.WriteString("\n")

	// Viewport with left padding
	viewportContent := m.viewport.View()
	b.WriteString(contentStyle.Render(viewportContent))
	b.WriteString("\n\n")

	// Input area with left padding and minimum width (only in PromptMode)
	if m.mode == PromptMode {
		textareaView := m.textarea.View()
		textareaStyled := lipgloss.NewStyle().
			Width(effectiveWidth).
			Border(lipgloss.NormalBorder(), true, false, true, false).
			BorderForeground(lipgloss.Color("240")).
			Padding(0, 1).
			Render(textareaView)
		b.WriteString(contentStyle.Render(textareaStyled))
		b.WriteString("\n")
	}

	// Bottom: Status line with Model, MSG count, and Timer (centered)
	modelStatus := fmt.Sprintf("Model: %s", m.currentModel)
	if !m.modelIsLoaded {
		modelStatus = fmt.Sprintf("Model: %s (not loaded)", m.currentModel)
	}

	// Calculate message count (1-based indexing for display)
	totalPairs := len(m.messagePairs)
	currentDisplay := 0
	if totalPairs > 0 {
		currentDisplay = m.currentPairIndex + 1
	}
	msgCount := fmt.Sprintf("MSG %d/%d", currentDisplay, totalPairs)

	var timerStr string
	if m.loadingModel {
		elapsed := time.Since(m.loadingStart)
		timerStr = fmt.Sprintf("⏱  Loading model: %.1fs", elapsed.Seconds())
	} else if m.isWaiting {
		elapsed := time.Since(m.waitingStart)
		timerStr = fmt.Sprintf("⏱  Waiting for response: %.1fs", elapsed.Seconds())
	}

	// Build status line with elements separated by spaces
	var statusParts []string
	statusParts = append(statusParts, modelStatus)
	statusParts = append(statusParts, msgCount)
	if timerStr != "" {
		statusParts = append(statusParts, timerStr)
	}

	statusLine := lipgloss.NewStyle().
		Foreground(lipgloss.Color("241")).
		Render(strings.Join(statusParts, " • "))

	b.WriteString(contentStyle.Render(statusLine))

	if m.err != nil {
		errStr := lipgloss.NewStyle().
			Foreground(lipgloss.Color("196")).
			Render(fmt.Sprintf("\nError: %v", m.err))
		b.WriteString(errStr)
	}

	return b.String()
}

// Commands
func tickCmd() tea.Cmd {
	return tea.Tick(100*time.Millisecond, func(t time.Time) tea.Msg {
		return tickMsg(t)
	})
}

func sendChatRequestCmd(messagePairs []MessagePair, modelName string) tea.Cmd {
	return func() tea.Msg {
		// Convert message pairs to Ollama messages
		var ollamaMessages []OllamaMessage
		for _, pair := range messagePairs {
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

		resp, err := http.Post(ollamaChatURL, "application/json", bytes.NewBuffer(jsonData))
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

		for scanner.Scan() {
			var streamResp StreamResponse
			if err := json.Unmarshal(scanner.Bytes(), &streamResp); err != nil {
				continue
			}

			if streamResp.Message.Content != "" {
				fullResponse.WriteString(streamResp.Message.Content)
			}
		}

		// Send the complete response
		content := fullResponse.String()
		return responseLineMsg(content + "\n")
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

// XDG_DATA_HOME functions
func getDataHome() string {
	dataHome := os.Getenv("XDG_DATA_HOME")
	if dataHome == "" {
		home, _ := os.UserHomeDir()
		dataHome = filepath.Join(home, ".local", "share")
	}
	return filepath.Join(dataHome, "tama")
}

func saveLastUsedModel(modelName string) error {
	dataDir := getDataHome()
	if err := os.MkdirAll(dataDir, 0755); err != nil {
		return err
	}

	filePath := filepath.Join(dataDir, "last-model")
	return os.WriteFile(filePath, []byte(modelName), 0644)
}

func loadLastUsedModel() string {
	filePath := filepath.Join(getDataHome(), "last-model")
	data, err := os.ReadFile(filePath)
	if err != nil {
		return defaultModel
	}
	model := strings.TrimSpace(string(data))
	if model == "" {
		return defaultModel
	}
	return model
}

func main() {
	p := tea.NewProgram(
		initialModel(),
		tea.WithAltScreen(),
		tea.WithMouseCellMotion(),
		tea.WithReportFocus(),
	)

	if _, err := p.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}
