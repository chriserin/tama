package main

import (
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/stretchr/testify/assert"
)

const LoremIpsum = `
 	Lorem ipsum dolor sit amet, consectetur adipiscing elit. Pellentesque euismod, nisi
  eu consectetur consectetur, nisl nunc varius nisi, nec lacinia lorem risus quis
  elit. Ut et risus eget odio interdum porta. Aenean non felis nec tortor pulvinar
  facilisis. Morbi tempor, nulla vel condimentum aliquet, sapien libero interdum
  mauris, in egestas metus orci at ligula.

  Vestibulum ante ipsum primis in faucibus orci luctus et ultrices posuere cubilia
  curae; Donec fermentum massa ut elit pulvinar, non tempor dui tempor. Integer et
  dui a justo interdum pulvinar. Quisque id nulla ac sem sodales condimentum. Etiam
  euismod, dolor sit amet facilisis placerat, turpis nulla condimentum sem, nec
  interdum libero justo nec purus.

  Proin id justo non lorem finibus commodo. Praesent sit amet lectus a justo volutpat
  tempor. Suspendisse potenti. Sed auctor, nulla at tempus tincidunt, lorem felis
  porta ex, non volutpat leo ex vel elit. Fusce sit amet justo ac sapien pulvinar
  ullamcorper.
`

// Scenario 1: Start in prompt mode
func TestStartInPromptMode(t *testing.T) {
	// Given a CLI prompt
	// When the user starts tama
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
	// Given a running tama in prompt mode
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
	// Given a running tama in read mode
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
	// Given a running tama
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

	// Then the top should display "TAMA"
	assert.Contains(t, view, "TAMA", "View should contain TAMA header")

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
	// Given a tama with a wide window (200 columns)
	m := initialModel()
	windowMsg := tea.WindowSizeMsg{Width: 200, Height: 30}
	updatedModel, _ := m.Update(windowMsg)
	m = updatedModel.(model)

	view := m.View()

	// The top and bottom should be centered with padding (like the content viewport)
	// effectiveWidth should be min(200, 100) = 100
	// leftPadding should be (200 - 100) / 2 = 50
	// The view should have leading spaces before TAMA and status line

	// Split into lines to check the first line (TAMA line)
	lines := strings.Split(view, "\n")
	assert.True(t, len(lines) > 0, "View should have lines")

	// The TAMA line should have leading spaces for centering
	tamaLine := lines[0]
	// With ANSI codes, we can't count exact spaces, but we can verify it's not at position 0
	// A properly centered line should have spaces before TAMA
	assert.True(t, strings.Index(tamaLine, "TAMA") > 0, "TAMA should be indented with padding")
}

// Scenario 5: Display message count and message indicator
func TestDisplayMessageCountAndIndicator(t *testing.T) {
	// Given a running tama in read mode
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
	// Given a running tama in read mode
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
	// Given a running tama in read mode
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

// Scenario 8: Scroll to top of current message pair
func TestScrollToTopOfCurrentMessagePair(t *testing.T) {
	// Given a running tama in read mode
	m := initialModel()
	windowMsg := tea.WindowSizeMsg{Width: 100, Height: 30}
	updatedModel, _ := m.Update(windowMsg)
	m = updatedModel.(model)

	// Switch to read mode
	escMsg := tea.KeyMsg{Type: tea.KeyEsc}
	updatedModel, _ = m.Update(escMsg)
	m = updatedModel.(model)

	// And the user has sent one request and received one response
	m.messagePairs = []MessagePair{
		{Request: "Test question", Response: "Test answer with a long response that might cause scrolling"},
	}
	m.currentPairIndex = 0
	m.updateViewport()

	// Scroll down in the viewport
	m.viewport.ScrollDown(5)

	// When the user presses "K"
	kMsg := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'K'}}
	updatedModel, _ = m.Update(kMsg)
	m = updatedModel.(model)

	// Then the viewport should be scrolled to the top
	assert.Equal(t, 0, m.viewport.YOffset, "Viewport should be scrolled to top (YOffset should be 0)")
}

// Scenario 9: Scroll to bottom of most recent message pair
func TestScrollToBottomOfMostRecentMessagePair(t *testing.T) {
	// Given a running tama in read mode
	m := initialModel()
	windowMsg := tea.WindowSizeMsg{Width: 100, Height: 30}
	updatedModel, _ := m.Update(windowMsg)
	m = updatedModel.(model)

	// Switch to read mode
	escMsg := tea.KeyMsg{Type: tea.KeyEsc}
	updatedModel, _ = m.Update(escMsg)
	m = updatedModel.(model)

	// And the user has sent one request and received one response
	m.messagePairs = []MessagePair{
		{Request: "Test question", Response: LoremIpsum + LoremIpsum}, // Long response to enable scrolling
	}
	m.currentPairIndex = 0
	m.updateViewport()

	// Start at the top
	m.viewport.GotoTop()
	initialOffset := m.viewport.YOffset

	// When the user presses "J"
	jMsg := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'J'}}
	updatedModel, _ = m.Update(jMsg)
	m = updatedModel.(model)

	// Then the viewport should be scrolled to the bottom
	assert.True(t, m.viewport.AtBottom(), "Viewport should be at bottom")
	assert.NotEqual(t, initialOffset, m.viewport.YOffset, "YOffset should have changed from initial position")
}

// Scenario 10: Move to next message pair
func TestMoveToNextMessagePair(t *testing.T) {
	// Given a running tama in read mode
	m := initialModel()
	windowMsg := tea.WindowSizeMsg{Width: 100, Height: 30}
	updatedModel, _ := m.Update(windowMsg)
	m = updatedModel.(model)

	// Switch to read mode
	escMsg := tea.KeyMsg{Type: tea.KeyEsc}
	updatedModel, _ = m.Update(escMsg)
	m = updatedModel.(model)

	// And the user has sent two request messages and received two responses
	m.messagePairs = []MessagePair{
		{Request: "First question", Response: "First answer"},
		{Request: "Second question", Response: "Second answer"},
	}

	// And the focus is on the first message pair
	m.currentPairIndex = 0
	m.updateViewport()

	// And already at the bottom
	m.viewport.GotoBottom()

	// When the user presses "J"
	jMsg := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'J'}}
	updatedModel, _ = m.Update(jMsg)
	m = updatedModel.(model)

	// Then the focus should move to the second message pair
	assert.Equal(t, 1, m.currentPairIndex, "Should move to second message pair")

	// And the viewport should show the second pair
	viewportContent := m.viewport.View()
	assert.Contains(t, viewportContent, "Second question", "Should show second message pair")
}

// Scenario 11: Move to previous message pair
func TestMoveToPreviousMessagePair(t *testing.T) {
	// Given a running tama in read mode
	m := initialModel()
	windowMsg := tea.WindowSizeMsg{Width: 100, Height: 30}
	updatedModel, _ := m.Update(windowMsg)
	m = updatedModel.(model)

	// Switch to read mode
	escMsg := tea.KeyMsg{Type: tea.KeyEsc}
	updatedModel, _ = m.Update(escMsg)
	m = updatedModel.(model)

	// And the user has sent two request messages and received two responses
	m.messagePairs = []MessagePair{
		{Request: "First question", Response: "First answer"},
		{Request: "Second question", Response: "Second answer"},
	}

	// And the focus is on the second message pair
	m.currentPairIndex = 1
	m.updateViewport()

	// And already at the top
	m.viewport.GotoTop()

	// When the user presses "K"
	kMsg := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'K'}}
	updatedModel, _ = m.Update(kMsg)
	m = updatedModel.(model)

	// Then the focus should move to the first message pair
	assert.Equal(t, 0, m.currentPairIndex, "Should move to first message pair")

	// And the viewport should show the first pair
	viewportContent := m.viewport.View()
	assert.Contains(t, viewportContent, "First question", "Should show first message pair")
}

// DAY 3 Feature Tests

// Scenario 12: Loaded response begins scrolled to top
func TestLoadedResponseBeginsScrolledToTop(t *testing.T) {
	// Given a running tama
	m := initialModel()
	windowMsg := tea.WindowSizeMsg{Width: 100, Height: 30}
	updatedModel, _ := m.Update(windowMsg)
	m = updatedModel.(model)

	// And the user has sent a request
	m.messagePairs = []MessagePair{
		{Request: "Test question", Response: ""},
	}
	m.currentPairIndex = 0

	// When a response is received and updateViewport is called
	m.messagePairs[0].Response = LoremIpsum + LoremIpsum // Long response
	m.messagePairs[0].Duration = 1 * time.Second
	m.updateViewport()

	// Then the viewport should be scrolled to the top
	assert.Equal(t, 0, m.viewport.YOffset, "Viewport should begin scrolled to top (YOffset should be 0)")
	assert.True(t, m.viewport.AtTop(), "Viewport should be at the top")
}

// Scenario 13: Go to Read Mode once a response is made
func TestGoToReadModeOnceResponseIsMade(t *testing.T) {
	// Given a running tama in prompt mode
	m := initialModel()
	windowMsg := tea.WindowSizeMsg{Width: 100, Height: 30}
	updatedModel, _ := m.Update(windowMsg)
	m = updatedModel.(model)

	assert.Equal(t, PromptMode, m.mode, "Should start in prompt mode")

	// And the user has sent a request
	m.messagePairs = []MessagePair{
		{Request: "Test question", Response: ""},
	}
	m.currentPairIndex = 0
	m.requestStart = time.Now()

	// When a response is received
	responseMsg := responseLineMsg("This is the response")
	updatedModel, _ = m.Update(responseMsg)
	m = updatedModel.(model)

	// Then the mode should switch to ReadMode
	assert.Equal(t, ReadMode, m.mode, "Should switch to ReadMode when response is received")
	assert.False(t, m.textarea.Focused(), "Textarea should not be focused in ReadMode")
}

// Scenario 14: Hide prompt box once in Read Mode
func TestHidePromptBoxInReadMode(t *testing.T) {
	// Given a running tama in read mode
	m := initialModel()
	windowMsg := tea.WindowSizeMsg{Width: 100, Height: 30}
	updatedModel, _ := m.Update(windowMsg)
	m = updatedModel.(model)

	// Switch to read mode
	escMsg := tea.KeyMsg{Type: tea.KeyEsc}
	updatedModel, _ = m.Update(escMsg)
	m = updatedModel.(model)

	assert.Equal(t, ReadMode, m.mode, "Should be in read mode")

	// When rendering the view
	view := m.View()

	// Then the prompt box should not be visible
	// The textarea view should not be in the output
	// We can check for the border and placeholder text
	assert.NotContains(t, view, "Type your message...", "Placeholder text should not be visible in ReadMode")

	// Switch back to prompt mode to compare
	m.mode = PromptMode
	m.textarea.Focus()
	promptModeView := m.View()

	// The views should be different (prompt mode shows more)
	assert.NotEqual(t, view, promptModeView, "View in ReadMode should be different from PromptMode")
	assert.True(t, len(promptModeView) > len(view), "PromptMode view should be longer (includes textarea)")
}

// Scenario 15: Show prompt box when exiting Read Mode
func TestShowPromptBoxWhenExitingReadMode(t *testing.T) {
	// Given a running tama in read mode
	m := initialModel()
	windowMsg := tea.WindowSizeMsg{Width: 100, Height: 30}
	updatedModel, _ := m.Update(windowMsg)
	m = updatedModel.(model)

	// Switch to read mode
	escMsg := tea.KeyMsg{Type: tea.KeyEsc}
	updatedModel, _ = m.Update(escMsg)
	m = updatedModel.(model)

	assert.Equal(t, ReadMode, m.mode, "Should be in read mode")

	// Verify textarea is not visible in ReadMode
	readModeView := m.View()
	assert.NotContains(t, readModeView, "Type your message...", "Placeholder should not be visible in ReadMode")

	// When the user presses "i" to exit read mode
	iMsg := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'i'}}
	updatedModel, _ = m.Update(iMsg)
	m = updatedModel.(model)

	// Then the mode should switch to PromptMode
	assert.Equal(t, PromptMode, m.mode, "Should switch to PromptMode")
	assert.True(t, m.textarea.Focused(), "Textarea should be focused")

	// And the prompt box should be visible
	promptModeView := m.View()
	assert.Contains(t, promptModeView, "Type your message...", "Placeholder should be visible in PromptMode")
}

// Scenario 16: Press G to go to bottom of response
func TestPressShiftGToGoToBottomOfResponse(t *testing.T) {
	// Given a running tama in read mode
	m := initialModel()
	windowMsg := tea.WindowSizeMsg{Width: 100, Height: 30}
	updatedModel, _ := m.Update(windowMsg)
	m = updatedModel.(model)

	// Switch to read mode
	escMsg := tea.KeyMsg{Type: tea.KeyEsc}
	updatedModel, _ = m.Update(escMsg)
	m = updatedModel.(model)

	// And the user has sent a request with a long response
	m.messagePairs = []MessagePair{
		{Request: "Test question", Response: LoremIpsum + LoremIpsum},
	}
	m.currentPairIndex = 0
	m.updateViewport()

	// Start at the top
	m.viewport.GotoTop()
	assert.True(t, m.viewport.AtTop(), "Should start at top")

	// When the user presses "G" (shift+g)
	gMsg := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'G'}}
	updatedModel, _ = m.Update(gMsg)
	m = updatedModel.(model)

	// Then the viewport should be at the bottom
	assert.True(t, m.viewport.AtBottom(), "Viewport should be at bottom after pressing G")
}

// Scenario 17: Press gg to go to top of response
func TestPressGGToGoToTopOfResponse(t *testing.T) {
	// Given a running tama in read mode
	m := initialModel()
	windowMsg := tea.WindowSizeMsg{Width: 100, Height: 30}
	updatedModel, _ := m.Update(windowMsg)
	m = updatedModel.(model)

	// Switch to read mode
	escMsg := tea.KeyMsg{Type: tea.KeyEsc}
	updatedModel, _ = m.Update(escMsg)
	m = updatedModel.(model)

	// And the user has sent a request with a long response
	m.messagePairs = []MessagePair{
		{Request: "Test question", Response: LoremIpsum + LoremIpsum},
	}
	m.currentPairIndex = 0
	m.updateViewport()

	// Start at the bottom
	m.viewport.GotoBottom()
	assert.True(t, m.viewport.AtBottom(), "Should start at bottom")

	// When the user presses "g" twice (gg)
	gMsg1 := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'g'}}
	updatedModel, _ = m.Update(gMsg1)
	m = updatedModel.(model)

	gMsg2 := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'g'}}
	updatedModel, _ = m.Update(gMsg2)
	m = updatedModel.(model)

	// Then the viewport should be at the top
	assert.True(t, m.viewport.AtTop(), "Viewport should be at top after pressing gg")
}

// Scenario 18: Test paragraph detection utility
func TestDetectParagraphBoundary(t *testing.T) {
	tests := []struct {
		name     string
		text     string
		expected bool
	}{
		{"Empty string", "", false},
		{"Single newline", "\n", false},
		{"Double newline", "\n\n", true},
		{"Text with double newline at end", "Some text\n\n", true},
		{"Text with single newline", "Some text\n", false},
		{"Multiple lines no double newline", "Line 1\nLine 2\nLine 3", false},
		{"Multiple lines with double newline", "Paragraph 1\n\nParagraph 2", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := hasParagraphBoundary(tt.text)
			assert.Equal(t, tt.expected, result, tt.name)
		})
	}
}
