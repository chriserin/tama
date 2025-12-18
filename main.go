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

	"github.com/charmbracelet/bubbles/cursor"
	"github.com/charmbracelet/bubbles/textarea"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/glamour"
	"github.com/charmbracelet/lipgloss"
)

// Ollama API types
type Message struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type ChatRequest struct {
	Model    string    `json:"model"`
	Messages []Message `json:"messages"`
	Stream   bool      `json:"stream"`
}

type ChatResponse struct {
	Model     string  `json:"model"`
	CreatedAt string  `json:"created_at"`
	Message   Message `json:"message"`
	Done      bool    `json:"done"`
}

type StreamResponse struct {
	Model     string  `json:"model"`
	CreatedAt string  `json:"created_at"`
	Message   Message `json:"message"`
	Done      bool    `json:"done"`
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

// Bubbletea messages
type tickMsg time.Time
type responseLineMsg string
type responseCompleteMsg struct{}
type errorMsg struct{ err error }
type modelLoadedMsg struct{ model string }
type modelStatusMsg struct{ loaded bool }

// Model holds the application state
type model struct {
	textarea      textarea.Model
	viewport      viewport.Model
	messages      []Message
	currentModel  string
	modelIsLoaded bool
	err           error
	width         int
	height        int
	ready         bool
	renderer      *glamour.TermRenderer
	loadingStart  time.Time
	waitingStart  time.Time
	isWaiting     bool
	chatRequested bool
	loadingModel  bool
	responseLines []string
	streamBuffer  string
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
		textarea:     ta,
		viewport:     vp,
		messages:     []Message{},
		currentModel: loadLastUsedModel(),
		renderer:     r,
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
		case tea.KeyCtrlC, tea.KeyEsc:
			return m, tea.Quit
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
				m.messages = []Message{}
				m.viewport.SetContent("")
				m.textarea.Reset()
				return m, nil
			}
			if input == "exit" || input == "quit" {
				return m, tea.Quit
			}

			// Add user message
			m.messages = append(m.messages, Message{
				Role:    "user",
				Content: input,
			})
			m.textarea.Reset()
			m.loadingModel = true
			m.modelIsLoaded = false
			m.loadingStart = time.Now()
			m.isWaiting = false
			m.chatRequested = true
			m.responseLines = []string{}
			m.streamBuffer = ""

			// Save the model being used
			saveLastUsedModel(m.currentModel)

			return m, tea.Batch(
				sendChatRequestCmd(m.messages, m.currentModel),
				checkModelStatus(m.currentModel),
				tickCmd(),
			)
		}

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height

		// Calculate effective width (at least contentWidth, or window width if smaller)
		effectiveWidth := min(m.width, contentWidth)

		if !m.ready {
			m.viewport = viewport.New(effectiveWidth, m.height-10)
			m.textarea.SetWidth(effectiveWidth - 4)
			m.ready = true
		} else {
			m.viewport.Width = effectiveWidth
			m.viewport.Height = m.height - 10
			m.textarea.SetWidth(effectiveWidth - 4)
		}

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
		// Response has started streaming, stop waiting timer
		m.isWaiting = false
		m.chatRequested = false

		line := string(msg)
		m.streamBuffer += line

		// Check if we have a complete line (ends with newline)
		if strings.HasSuffix(line, "\n") {
			// Render the complete line as markdown
			rendered, err := m.renderer.Render(m.streamBuffer)
			if err != nil {
				m.responseLines = append(m.responseLines, m.streamBuffer)
			} else {
				m.responseLines = append(m.responseLines, rendered)
			}
			m.streamBuffer = ""
			m.updateViewport()
		}

	case responseCompleteMsg:
		m.isWaiting = false
		// Render any remaining content in the buffer
		if m.streamBuffer != "" {
			rendered, err := m.renderer.Render(m.streamBuffer)
			if err != nil {
				m.responseLines = append(m.responseLines, m.streamBuffer)
			} else {
				m.responseLines = append(m.responseLines, rendered)
			}
			m.streamBuffer = ""
		}
		m.updateViewport()

	case modelLoadedMsg:
		// Model selected (not necessarily loaded yet)
		m.currentModel = msg.model
		saveLastUsedModel(m.currentModel)
		// Don't set modelIsLoaded - let modelStatusMsg handle that

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

	m.textarea, cmd = m.textarea.Update(msg)
	lines := m.textarea.LineInfo().Height
	m.textarea.SetHeight(lines)
	//key msg for InputEnd
	inputStartKeyMsg := tea.KeyMsg{Type: tea.KeyCtrlA}
	inputEndKeyMsg := tea.KeyMsg{Type: tea.KeyCtrlE}
	m.textarea, _ = m.textarea.Update(inputStartKeyMsg)
	m.textarea, _ = m.textarea.Update(inputEndKeyMsg)
	cmds = append(cmds, cmd)

	m.viewport, cmd = m.viewport.Update(msg)
	cmds = append(cmds, cmd)

	return m, tea.Batch(cmds...)
}

func (m *model) updateViewport() {
	content := strings.Join(m.responseLines, "")
	m.viewport.SetContent(content)
	m.viewport.GotoBottom()
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

	// Header with model name
	modelStatus := m.currentModel
	if !m.modelIsLoaded {
		modelStatus += " (not loaded)"
	}
	header := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("205")).
		Render(fmt.Sprintf("Ollama Chat REPL - Current Model: %s", modelStatus))

	// Timer display
	var timerStr string
	if m.loadingModel {
		elapsed := time.Since(m.loadingStart)
		timerStr = lipgloss.NewStyle().
			Foreground(lipgloss.Color("241")).
			Render(fmt.Sprintf("⏱  Loading model: %.1fs", elapsed.Seconds()))
	} else if m.isWaiting {
		elapsed := time.Since(m.waitingStart)
		timerStr = lipgloss.NewStyle().
			Foreground(lipgloss.Color("241")).
			Render(fmt.Sprintf("⏱  Waiting for response: %.1fs", elapsed.Seconds()))
	}

	// Apply padding to center content
	b.WriteString(contentStyle.Render(header))
	b.WriteString("\n")
	if timerStr != "" {
		b.WriteString(contentStyle.Render(timerStr))
		b.WriteString("\n")
	}
	b.WriteString("\n")

	// Viewport with left padding
	viewportContent := m.viewport.View()
	b.WriteString(contentStyle.Render(viewportContent))
	b.WriteString("\n\n")

	// Input area with left padding and minimum width
	textareaView := m.textarea.View()
	textareaStyled := lipgloss.NewStyle().
		Width(effectiveWidth).
		Border(lipgloss.NormalBorder(), true, false, true, false).
		BorderForeground(lipgloss.Color("240")).
		Padding(0, 1).
		Render(textareaView)
	b.WriteString(contentStyle.Render(textareaStyled))

	// Help text
	help := lipgloss.NewStyle().
		Foreground(lipgloss.Color("241")).
		Render("\nEnter: send • Ctrl+C: quit • 'clear': clear history")
	b.WriteString(contentStyle.Render(help))

	if m.err != nil {
		errStr := lipgloss.NewStyle().
			Foreground(lipgloss.Color("196")).
			Render(fmt.Sprintf("\nError: %v", m.err))
		b.WriteString(contentStyle.Render(errStr))
	}

	return b.String()
}

// Commands
func tickCmd() tea.Cmd {
	return tea.Tick(100*time.Millisecond, func(t time.Time) tea.Msg {
		return tickMsg(t)
	})
}

func sendChatRequestCmd(messages []Message, modelName string) tea.Cmd {
	return func() tea.Msg {
		reqBody := ChatRequest{
			Model:    modelName,
			Messages: messages,
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
			return modelLoadedMsg{model: lastModel}
		}

		return modelLoadedMsg{model: defaultModel}
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
	return filepath.Join(dataHome, "rama")
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
