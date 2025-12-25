package tui

import (
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
	Request   string
	Response  string
	Duration  time.Duration // Time taken to generate the response
	Cancelled bool          // Whether the request was cancelled
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
	ContentWidth    = 100
)

// Mode represents the current interaction mode
type Mode int

const (
	PromptMode Mode = iota
	ReadMode
)

// Bubbletea messages
type tickMsg time.Time
type ResponseLineMsg string
type ResponseCompleteMsg string
type errorMsg struct{ err error }
type modelLoadedMsg struct{ model string }
type modelSelectedMsg struct{ model string }
type modelStatusMsg struct{ loaded bool }
type SetSendFuncMsg struct{ Send func(tea.Msg) }

// Model holds the application state
type Model struct {
	Mode                   Mode
	Textarea               textarea.Model
	Viewport               viewport.Model
	MessagePairs           []MessagePair
	CurrentPairIndex       int // 0-based index of currently focused message pair
	CurrentModel           string
	ModelIsLoaded          bool
	Err                    error
	Width                  int
	Height                 int
	Ready                  bool
	Renderer               *glamour.TermRenderer
	LoadingStart           time.Time
	WaitingStart           time.Time
	RequestStart           time.Time // Time when current request was sent
	IsWaiting              bool
	ChatRequested          bool
	LoadingModel           bool
	ResponseLines          []string
	StreamBuffer           string
	LastKeyWasG            bool          // Track if last key pressed was 'g' for 'gg' sequence
	Send                   func(tea.Msg) // Function to send messages to the program
	cancelCurrentRequestFn func()        // Function to cancel the current request
	ChatURL                string        // Ollama chat API URL (configurable for testing)
	ResponseTargetIndex    int           // Index of message pair currently receiving response
}

func InitialModel() Model {
	ta := textarea.New()
	ta.Placeholder = "Type your message..."
	ta.Focus()
	ta.CharLimit = 0
	ta.SetWidth(ContentWidth - 4)
	ta.SetHeight(1)
	ta.ShowLineNumbers = false
	ta.FocusedStyle.CursorLine = lipgloss.NewStyle()
	ta.Cursor.SetMode(cursor.CursorStatic)

	vp := viewport.New(ContentWidth, 20)

	// Initialize markdown renderer
	r, _ := glamour.NewTermRenderer(
		glamour.WithStandardStyle("tokyo-night"),
		glamour.WithWordWrap(ContentWidth),
	)

	return Model{
		Mode:             PromptMode,
		Textarea:         ta,
		Viewport:         vp,
		MessagePairs:     []MessagePair{},
		CurrentPairIndex: 0,
		CurrentModel:     loadLastUsedModel(),
		Renderer:         r,
		ChatURL:          ollamaChatURL,
	}
}

// Helper functions
func HasParagraphBoundary(text string) bool {
	return strings.Contains(text, "\n\n")
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
