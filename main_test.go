package main

import (
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/stretchr/testify/assert"
)

// Scenario 1: Start in prompt mode
func TestStartInPromptMode(t *testing.T) {
	// Given a CLI prompt
	// When the user starts rama
	m := initialModel()

	// Then the interface should be in prompt mode
	assert.Equal(t, PromptMode, m.mode, "Interface should start in prompt mode")

	// And the user should see a prompt input
	assert.True(t, m.textarea.Focused(), "Textarea should be focused")

	// And all keystrokes should be directed to the prompt input
	// (This is implicitly tested by the textarea being focused)
}

// Scenario 2: Exit prompt mode / Enter read mode
func TestExitPromptModeEnterReadMode(t *testing.T) {
	// Given a running rama in prompt mode
	m := initialModel()
	assert.Equal(t, PromptMode, m.mode)

	// When the user presses "Esc"
	escMsg := tea.KeyMsg{Type: tea.KeyEsc}
	updatedModel, _ := m.Update(escMsg)
	m = updatedModel.(model)

	// Then the interface should switch to read mode
	assert.Equal(t, ReadMode, m.mode, "Should switch to read mode")

	// And the user should not see a prompt input (textarea not focused)
	assert.False(t, m.textarea.Focused(), "Textarea should not be focused in read mode")

	// And all keystrokes should be directed to message navigation
	// (This will be tested when navigation is implemented)
}

// Scenario 3: Enter prompt mode / Exit read mode
func TestEnterPromptModeExitReadMode(t *testing.T) {
	// Given a running rama in read mode
	m := initialModel()
	// First switch to read mode
	escMsg := tea.KeyMsg{Type: tea.KeyEsc}
	updatedModel, _ := m.Update(escMsg)
	m = updatedModel.(model)
	assert.Equal(t, ReadMode, m.mode, "Should start in read mode for this test")

	// When the user presses "i"
	iMsg := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'i'}}
	updatedModel, _ = m.Update(iMsg)
	m = updatedModel.(model)

	// Then the interface should switch to prompt mode
	assert.Equal(t, PromptMode, m.mode, "Should switch to prompt mode")

	// And the user should see a prompt input (textarea focused)
	assert.True(t, m.textarea.Focused(), "Textarea should be focused in prompt mode")

	// And all keystrokes should be directed to the prompt input
	// (This is implicitly tested by the textarea being focused)
}

// Scenario 4: Top and bottom of application
func TestTopAndBottomOfApplication(t *testing.T) {
	// Given a running rama
	m := initialModel()
	m.ready = true
	m.width = 100
	m.height = 30

	// Initialize viewport to make model ready
	windowMsg := tea.WindowSizeMsg{Width: 100, Height: 30}
	updatedModel, _ := m.Update(windowMsg)
	m = updatedModel.(model)

	// Get the view
	view := m.View()

	// Then the top should display "RAMA"
	assert.Contains(t, view, "RAMA", "View should contain RAMA header")

	// And a horizontal line stretching across the width
	assert.Contains(t, view, "─", "View should contain horizontal line")

	// And the bottom should show the status line with Model
	assert.Contains(t, view, "Model:", "Status line should contain Model")

	// And MSG count
	assert.Contains(t, view, "MSG", "Status line should contain MSG count")

	// When a timer is active, it should be shown
	m.loadingModel = true
	m.loadingStart = time.Now()
	view = m.View()
	assert.Contains(t, view, "Loading model", "Status line should show loading timer when active")
}

// Scenario 4b: Top and bottom respond to width like content
func TestTopAndBottomRespondToWidth(t *testing.T) {
	// Given a rama with a wide window (200 columns)
	m := initialModel()
	windowMsg := tea.WindowSizeMsg{Width: 200, Height: 30}
	updatedModel, _ := m.Update(windowMsg)
	m = updatedModel.(model)

	view := m.View()

	// The top and bottom should be centered with padding (like the content viewport)
	// effectiveWidth should be min(200, 100) = 100
	// leftPadding should be (200 - 100) / 2 = 50
	// The view should have leading spaces before RAMA and status line

	// Split into lines to check the first line (RAMA line)
	lines := strings.Split(view, "\n")
	assert.True(t, len(lines) > 0, "View should have lines")

	// The RAMA line should have leading spaces for centering
	ramaLine := lines[0]
	// With ANSI codes, we can't count exact spaces, but we can verify it's not at position 0
	// A properly centered line should have spaces before RAMA
	assert.True(t, strings.Index(ramaLine, "RAMA") > 0, "RAMA should be indented with padding")
}

// Scenario 5: Display message count and message indicator
func TestDisplayMessageCountAndIndicator(t *testing.T) {
	// Given a running rama in read mode
	m := initialModel()
	windowMsg := tea.WindowSizeMsg{Width: 100, Height: 30}
	updatedModel, _ := m.Update(windowMsg)
	m = updatedModel.(model)

	// And the user has sent three request messages and received three responses
	m.messagePairs = []MessagePair{
		{Request: "First question", Response: "First answer"},
		{Request: "Second question", Response: "Second answer"},
		{Request: "Third question", Response: "Third answer"},
	}

	// When the focus is on the second message pair (index 1)
	m.currentPairIndex = 1

	// Get the view
	view := m.View()

	// Then the status bar should display "MSG 2/3"
	assert.Contains(t, view, "MSG 2/3", "Status bar should display MSG 2/3")
}

// Scenario 6: Request message surrounded in border
func TestRequestMessageSurroundedInBorder(t *testing.T) {
	// Given a running rama in read mode
	m := initialModel()
	windowMsg := tea.WindowSizeMsg{Width: 100, Height: 30}
	updatedModel, _ := m.Update(windowMsg)
	m = updatedModel.(model)

	// Switch to read mode
	escMsg := tea.KeyMsg{Type: tea.KeyEsc}
	updatedModel, _ = m.Update(escMsg)
	m = updatedModel.(model)

	// When the user has sent a request message
	m.messagePairs = []MessagePair{
		{Request: "Test question", Response: "Test answer", Duration: 2500 * time.Millisecond},
	}
	m.currentPairIndex = 0
	m.updateViewport()

	// Get the viewport content
	viewportContent := m.viewport.View()

	// Then the request message should be displayed with a top border
	assert.Contains(t, viewportContent, "Request", "Top border should contain 'Request'")

	// And the word "Response" should be shown in the bottom border
	assert.Contains(t, viewportContent, "Response", "Bottom border should contain 'Response'")

	// And the duration should be shown in the bottom border
	assert.Contains(t, viewportContent, "2.5s", "Bottom border should show duration")

	// And no "You:"/"Assistant:" labels should be shown
	assert.NotContains(t, viewportContent, "You:", "Should not contain 'You:' label")
	assert.NotContains(t, viewportContent, "Assistant:", "Should not contain 'Assistant:' label")
}

// Scenario 7: Display one message pair at a time
func TestDisplayOneMessagePairAtATime(t *testing.T) {
	// Given a running rama in read mode
	m := initialModel()
	windowMsg := tea.WindowSizeMsg{Width: 100, Height: 30}
	updatedModel, _ := m.Update(windowMsg)
	m = updatedModel.(model)

	// And the user has sent a request and received a response
	m.messagePairs = []MessagePair{
		{Request: "What is Go?", Response: "Go is a programming language"},
	}
	m.currentPairIndex = 0
	m.updateViewport()

	// And this message pair is visible in the viewport
	viewportContent := m.viewport.View()
	assert.Contains(t, viewportContent, "What is Go?", "First message pair should be visible")

	// When the user sends a second request and receives a second response
	m.messagePairs = append(m.messagePairs, MessagePair{
		Request:  "What is Rust?",
		Response: "Rust is a systems language",
	})
	m.currentPairIndex = 1 // Focus on second pair
	m.updateViewport()

	// Then only the second request and response messages should be visible
	viewportContent = m.viewport.View()
	assert.Contains(t, viewportContent, "What is Rust?", "Second message should be visible")
	assert.Contains(t, viewportContent, "Rust", "Second response should contain 'Rust'")
	assert.NotContains(t, viewportContent, "What is Go?", "First message should not be visible")
	assert.NotContains(t, viewportContent, "programming language", "First response should not be visible")
}
