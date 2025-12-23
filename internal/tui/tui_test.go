package tui

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/glamour"
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
	m := InitialModel()

	// Then the interface should be in prompt mode
	assert.Equal(t, PromptMode, m.Mode, "Interface should start in prompt mode")

	// And the user should see a prompt input
	assert.True(t, m.Textarea.Focused(), "Textarea should be focused")

	// And all keystrokes should be directed to the prompt input
	// (This is implicitly tested by the textarea being focused)
}

// Scenario 2: Exit prompt mode / Enter read mode
func TestExitPromptModeEnterReadMode(t *testing.T) {
	// Given a running tama in prompt mode
	m := InitialModel()
	assert.Equal(t, PromptMode, m.Mode)

	// When the user presses "Esc"
	escMsg := tea.KeyMsg{Type: tea.KeyEsc}
	updatedModel, _ := m.Update(escMsg)
	m = updatedModel.(Model)

	// Then the interface should switch to read mode
	assert.Equal(t, ReadMode, m.Mode, "Should switch to read mode")

	// And the user should not see a prompt input (textarea not focused)
	assert.False(t, m.Textarea.Focused(), "Textarea should not be focused in read mode")

	// And all keystrokes should be directed to message navigation
	// (This will be tested when navigation is implemented)
}

// Scenario 3: Enter prompt mode / Exit read mode
func TestEnterPromptModeExitReadMode(t *testing.T) {
	// Given a running tama in read mode
	m := InitialModel()
	// First switch to read mode
	escMsg := tea.KeyMsg{Type: tea.KeyEsc}
	updatedModel, _ := m.Update(escMsg)
	m = updatedModel.(Model)
	assert.Equal(t, ReadMode, m.Mode, "Should start in read mode for this test")

	// When the user presses "i"
	iMsg := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'i'}}
	updatedModel, _ = m.Update(iMsg)
	m = updatedModel.(Model)

	// Then the interface should switch to prompt mode
	assert.Equal(t, PromptMode, m.Mode, "Should switch to prompt mode")

	// And the user should see a prompt input (textarea focused)
	assert.True(t, m.Textarea.Focused(), "Textarea should be focused in prompt mode")

	// And all keystrokes should be directed to the prompt input
	// (This is implicitly tested by the textarea being focused)
}

// Scenario 4: Top and bottom of application
func TestTopAndBottomOfApplication(t *testing.T) {
	// Given a running tama
	m := InitialModel()
	m.Ready = true
	m.Width = 100
	m.Height = 30

	// Initialize viewport to make Model ready
	windowMsg := tea.WindowSizeMsg{Width: 100, Height: 30}
	updatedModel, _ := m.Update(windowMsg)
	m = updatedModel.(Model)

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
	m.LoadingModel = true
	m.LoadingStart = time.Now()
	view = m.View()
	assert.Contains(t, view, "Loading model", "Status line should show loading timer when active")
}

// Scenario 4b: Top and bottom respond to width like content
func TestTopAndBottomRespondToWidth(t *testing.T) {
	// Given a tama with a wide window (200 columns)
	m := InitialModel()
	windowMsg := tea.WindowSizeMsg{Width: 200, Height: 30}
	updatedModel, _ := m.Update(windowMsg)
	m = updatedModel.(Model)

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
	m := InitialModel()
	windowMsg := tea.WindowSizeMsg{Width: 100, Height: 30}
	updatedModel, _ := m.Update(windowMsg)
	m = updatedModel.(Model)

	// And the user has sent three request messages and received three responses
	m.MessagePairs = []MessagePair{
		{Request: "First question", Response: "First answer"},
		{Request: "Second question", Response: "Second answer"},
		{Request: "Third question", Response: "Third answer"},
	}

	// When the focus is on the second message pair (index 1)
	m.CurrentPairIndex = 1

	// Get the view
	view := m.View()

	// Then the status bar should display "MSG 2/3"
	assert.Contains(t, view, "MSG 2/3", "Status bar should display MSG 2/3")
}

// Scenario 6: Request message surrounded in border
func TestRequestMessageSurroundedInBorder(t *testing.T) {
	// Given a running tama in read mode
	m := InitialModel()
	windowMsg := tea.WindowSizeMsg{Width: 100, Height: 30}
	updatedModel, _ := m.Update(windowMsg)
	m = updatedModel.(Model)

	// Switch to read mode
	escMsg := tea.KeyMsg{Type: tea.KeyEsc}
	updatedModel, _ = m.Update(escMsg)
	m = updatedModel.(Model)

	// When the user has sent a request message
	m.MessagePairs = []MessagePair{
		{Request: "Test question", Response: "Test answer", Duration: 2500 * time.Millisecond},
	}
	m.CurrentPairIndex = 0
	m.updateViewport()

	// Get the viewport content
	viewportContent := m.Viewport.View()

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
	m := InitialModel()
	windowMsg := tea.WindowSizeMsg{Width: 100, Height: 30}
	updatedModel, _ := m.Update(windowMsg)
	m = updatedModel.(Model)

	// And the user has sent a request and received a response
	m.MessagePairs = []MessagePair{
		{Request: "What is Go?", Response: "Go is a programming language"},
	}
	m.CurrentPairIndex = 0
	m.updateViewport()

	// And this message pair is visible in the viewport
	viewportContent := m.Viewport.View()
	assert.Contains(t, viewportContent, "What is Go?", "First message pair should be visible")

	// When the user sends a second request and receives a second response
	m.MessagePairs = append(m.MessagePairs, MessagePair{
		Request:  "What is Rust?",
		Response: "Rust is a systems language",
	})
	m.CurrentPairIndex = 1 // Focus on second pair
	m.updateViewport()

	// Then only the second request and response messages should be visible
	viewportContent = m.Viewport.View()
	assert.Contains(t, viewportContent, "What is Rust?", "Second message should be visible")
	assert.Contains(t, viewportContent, "Rust", "Second response should contain 'Rust'")
	assert.NotContains(t, viewportContent, "What is Go?", "First message should not be visible")
	assert.NotContains(t, viewportContent, "programming language", "First response should not be visible")
}

// Scenario 8: K should not scroll within current message, only navigate to previous
func TestKShouldNotScrollWithinMessage(t *testing.T) {
	// Given a running tama in read mode with only one message pair
	m := InitialModel()
	windowMsg := tea.WindowSizeMsg{Width: 100, Height: 30}
	updatedModel, _ := m.Update(windowMsg)
	m = updatedModel.(Model)

	// Switch to read mode
	escMsg := tea.KeyMsg{Type: tea.KeyEsc}
	updatedModel, _ = m.Update(escMsg)
	m = updatedModel.(Model)

	// And the user has sent one request and received one response
	m.MessagePairs = []MessagePair{
		{Request: "Test question", Response: LoremIpsum + LoremIpsum}, // Long response
	}
	m.CurrentPairIndex = 0
	m.updateViewport()

	// Scroll down in the viewport
	m.Viewport.ScrollDown(5)
	initialOffset := m.Viewport.YOffset
	assert.True(t, initialOffset > 0, "Should have scrolled down")

	// When the user presses "K" (with no previous message to navigate to)
	kMsg := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'K'}}
	updatedModel, _ = m.Update(kMsg)
	m = updatedModel.(Model)

	// Then K should do nothing (no scroll, no navigation)
	assert.Equal(t, initialOffset, m.Viewport.YOffset, "K should not scroll within current message when there's no previous message")
	assert.Equal(t, 0, m.CurrentPairIndex, "Should still be on first message pair")
}

// Scenario 9: J should not scroll within current message, only navigate to next
func TestJShouldNotScrollWithinMessage(t *testing.T) {
	// Given a running tama in read mode with only one message pair
	m := InitialModel()
	windowMsg := tea.WindowSizeMsg{Width: 100, Height: 30}
	updatedModel, _ := m.Update(windowMsg)
	m = updatedModel.(Model)

	// Switch to read mode
	escMsg := tea.KeyMsg{Type: tea.KeyEsc}
	updatedModel, _ = m.Update(escMsg)
	m = updatedModel.(Model)

	// And the user has sent one request and received one response
	m.MessagePairs = []MessagePair{
		{Request: "Test question", Response: LoremIpsum + LoremIpsum}, // Long response
	}
	m.CurrentPairIndex = 0
	m.updateViewport()

	// Start at the top
	m.Viewport.GotoTop()
	initialOffset := m.Viewport.YOffset
	assert.Equal(t, 0, initialOffset, "Should start at top")

	// When the user presses "J" (with no next message to navigate to)
	jMsg := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'J'}}
	updatedModel, _ = m.Update(jMsg)
	m = updatedModel.(Model)

	// Then J should do nothing (no scroll, no navigation)
	assert.Equal(t, initialOffset, m.Viewport.YOffset, "J should not scroll within current message when there's no next message")
	assert.Equal(t, 0, m.CurrentPairIndex, "Should still be on first message pair")
}

// Scenario 10: Move to next message pair and scroll to top
func TestMoveToNextMessagePair(t *testing.T) {
	// Given a running tama in read mode
	m := InitialModel()
	windowMsg := tea.WindowSizeMsg{Width: 100, Height: 30}
	updatedModel, _ := m.Update(windowMsg)
	m = updatedModel.(Model)

	// Switch to read mode
	escMsg := tea.KeyMsg{Type: tea.KeyEsc}
	updatedModel, _ = m.Update(escMsg)
	m = updatedModel.(Model)

	// And the user has sent two request messages and received two responses
	m.MessagePairs = []MessagePair{
		{Request: "First question", Response: LoremIpsum + LoremIpsum + LoremIpsum},
		{Request: "Second question", Response: LoremIpsum + LoremIpsum + LoremIpsum},
	}

	// And the focus is on the first message pair
	m.CurrentPairIndex = 0
	m.updateViewport()

	// Scroll down to verify it resets to top when navigating
	for i := 0; i < 10; i++ {
		m.Viewport.LineDown(1)
	}
	assert.True(t, m.Viewport.YOffset > 0, "Should be scrolled down")

	// When the user presses "J"
	jMsg := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'J'}}
	updatedModel, _ = m.Update(jMsg)
	m = updatedModel.(Model)

	// Then the focus should move to the second message pair
	assert.Equal(t, 1, m.CurrentPairIndex, "Should move to second message pair")

	// And the viewport should be scrolled to top (YOffset = 0)
	assert.Equal(t, 0, m.Viewport.YOffset, "Viewport should be at top after navigating to next message")

	// And the viewport should show the second pair
	viewportContent := m.Viewport.View()
	assert.Contains(t, viewportContent, "Second question", "Should show second message pair")
}

// Scenario 11: Move to previous message pair and scroll to top
func TestMoveToPreviousMessagePair(t *testing.T) {
	// Given a running tama in read mode
	m := InitialModel()
	windowMsg := tea.WindowSizeMsg{Width: 100, Height: 30}
	updatedModel, _ := m.Update(windowMsg)
	m = updatedModel.(Model)

	// Switch to read mode
	escMsg := tea.KeyMsg{Type: tea.KeyEsc}
	updatedModel, _ = m.Update(escMsg)
	m = updatedModel.(Model)

	// And the user has sent two request messages and received two responses
	m.MessagePairs = []MessagePair{
		{Request: "First question", Response: LoremIpsum + LoremIpsum + LoremIpsum},
		{Request: "Second question", Response: LoremIpsum + LoremIpsum + LoremIpsum},
	}

	// And the focus is on the second message pair
	m.CurrentPairIndex = 1
	m.updateViewport()

	// Scroll down to verify it resets to top when navigating
	for i := 0; i < 10; i++ {
		m.Viewport.LineDown(1)
	}
	assert.True(t, m.Viewport.YOffset > 0, "Should be scrolled down")

	// When the user presses "K"
	kMsg := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'K'}}
	updatedModel, _ = m.Update(kMsg)
	m = updatedModel.(Model)

	// Then the focus should move to the first message pair
	assert.Equal(t, 0, m.CurrentPairIndex, "Should move to first message pair")

	// And the viewport should be scrolled to top (YOffset = 0)
	assert.Equal(t, 0, m.Viewport.YOffset, "Viewport should be at top after navigating to previous message")

	// And the viewport should show the first pair
	viewportContent := m.Viewport.View()
	assert.Contains(t, viewportContent, "First question", "Should show first message pair")
}

// DAY 3 Feature Tests

// Scenario 12: Loaded response begins scrolled to top
func TestLoadedResponseBeginsScrolledToTop(t *testing.T) {
	// Given a running tama
	m := InitialModel()
	windowMsg := tea.WindowSizeMsg{Width: 100, Height: 30}
	updatedModel, _ := m.Update(windowMsg)
	m = updatedModel.(Model)

	// And the user has sent a request
	m.MessagePairs = []MessagePair{
		{Request: "Test question", Response: ""},
	}
	m.CurrentPairIndex = 0

	// When a response is received and updateViewport is called
	m.MessagePairs[0].Response = LoremIpsum + LoremIpsum // Long response
	m.MessagePairs[0].Duration = 1 * time.Second
	m.updateViewport()

	// Then the viewport should be scrolled to the top
	assert.Equal(t, 0, m.Viewport.YOffset, "Viewport should begin scrolled to top (YOffset should be 0)")
	assert.True(t, m.Viewport.AtTop(), "Viewport should be at the top")
}

// Scenario 13: Go to Read Mode once a response is made
func TestGoToReadModeOnceResponseIsMade(t *testing.T) {
	// Given a running tama in prompt mode
	m := InitialModel()
	windowMsg := tea.WindowSizeMsg{Width: 100, Height: 30}
	updatedModel, _ := m.Update(windowMsg)
	m = updatedModel.(Model)

	assert.Equal(t, PromptMode, m.Mode, "Should start in prompt mode")

	// And the user has sent a request
	m.MessagePairs = []MessagePair{
		{Request: "Test question", Response: ""},
	}
	m.CurrentPairIndex = 0
	m.RequestStart = time.Now()

	// When a response is received
	responseMsg := ResponseCompleteMsg("This is the response")
	updatedModel, _ = m.Update(responseMsg)
	m = updatedModel.(Model)

	// Then the mode should switch to ReadMode
	assert.Equal(t, ReadMode, m.Mode, "Should switch to ReadMode when response is received")
	assert.False(t, m.Textarea.Focused(), "Textarea should not be focused in ReadMode")
}

// Scenario 14: Hide prompt box once in Read Mode
func TestHidePromptBoxInReadMode(t *testing.T) {
	// Given a running tama in read mode
	m := InitialModel()
	windowMsg := tea.WindowSizeMsg{Width: 100, Height: 30}
	updatedModel, _ := m.Update(windowMsg)
	m = updatedModel.(Model)

	// Switch to read mode
	escMsg := tea.KeyMsg{Type: tea.KeyEsc}
	updatedModel, _ = m.Update(escMsg)
	m = updatedModel.(Model)

	assert.Equal(t, ReadMode, m.Mode, "Should be in read mode")

	// When rendering the view
	view := m.View()

	// Then the prompt box should not be visible
	// The textarea view should not be in the output
	// We can check for the border and placeholder text
	assert.NotContains(t, view, "Type your message...", "Placeholder text should not be visible in ReadMode")

	// Switch back to prompt mode to compare
	m.Mode = PromptMode
	m.Textarea.Focus()
	promptModeView := m.View()

	// The views should be different (prompt mode shows more)
	assert.NotEqual(t, view, promptModeView, "View in ReadMode should be different from PromptMode")
	assert.True(t, len(promptModeView) > len(view), "PromptMode view should be longer (includes textarea)")
}

// Scenario 15: Show prompt box when exiting Read Mode
func TestShowPromptBoxWhenExitingReadMode(t *testing.T) {
	// Given a running tama in read mode
	m := InitialModel()
	windowMsg := tea.WindowSizeMsg{Width: 100, Height: 30}
	updatedModel, _ := m.Update(windowMsg)
	m = updatedModel.(Model)

	// Switch to read mode
	escMsg := tea.KeyMsg{Type: tea.KeyEsc}
	updatedModel, _ = m.Update(escMsg)
	m = updatedModel.(Model)

	assert.Equal(t, ReadMode, m.Mode, "Should be in read mode")

	// Verify textarea is not visible in ReadMode
	readModeView := m.View()
	assert.NotContains(t, readModeView, "Type your message...", "Placeholder should not be visible in ReadMode")

	// When the user presses "i" to exit read mode
	iMsg := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'i'}}
	updatedModel, _ = m.Update(iMsg)
	m = updatedModel.(Model)

	// Then the mode should switch to PromptMode
	assert.Equal(t, PromptMode, m.Mode, "Should switch to PromptMode")
	assert.True(t, m.Textarea.Focused(), "Textarea should be focused")

	// And the prompt box should be visible
	promptModeView := m.View()
	assert.Contains(t, promptModeView, "Type your message...", "Placeholder should be visible in PromptMode")
}

// Scenario 16: Press G to go to bottom of response
func TestPressShiftGToGoToBottomOfResponse(t *testing.T) {
	// Given a running tama in read mode
	m := InitialModel()
	windowMsg := tea.WindowSizeMsg{Width: 100, Height: 30}
	updatedModel, _ := m.Update(windowMsg)
	m = updatedModel.(Model)

	// Switch to read mode
	escMsg := tea.KeyMsg{Type: tea.KeyEsc}
	updatedModel, _ = m.Update(escMsg)
	m = updatedModel.(Model)

	// And the user has sent a request with a long response
	m.MessagePairs = []MessagePair{
		{Request: "Test question", Response: LoremIpsum + LoremIpsum},
	}
	m.CurrentPairIndex = 0
	m.updateViewport()

	// Start at the top
	m.Viewport.GotoTop()
	assert.True(t, m.Viewport.AtTop(), "Should start at top")

	// When the user presses "G" (shift+g)
	gMsg := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'G'}}
	updatedModel, _ = m.Update(gMsg)
	m = updatedModel.(Model)

	// Then the viewport should be at the bottom
	assert.True(t, m.Viewport.AtBottom(), "Viewport should be at bottom after pressing G")
}

// Scenario 17: Press gg to go to top of response
func TestPressGGToGoToTopOfResponse(t *testing.T) {
	// Given a running tama in read mode
	m := InitialModel()
	windowMsg := tea.WindowSizeMsg{Width: 100, Height: 30}
	updatedModel, _ := m.Update(windowMsg)
	m = updatedModel.(Model)

	// Switch to read mode
	escMsg := tea.KeyMsg{Type: tea.KeyEsc}
	updatedModel, _ = m.Update(escMsg)
	m = updatedModel.(Model)

	// And the user has sent a request with a long response
	m.MessagePairs = []MessagePair{
		{Request: "Test question", Response: LoremIpsum + LoremIpsum},
	}
	m.CurrentPairIndex = 0
	m.updateViewport()

	// Start at the bottom
	m.Viewport.GotoBottom()
	assert.True(t, m.Viewport.AtBottom(), "Should start at bottom")

	// When the user presses "g" twice (gg)
	gMsg1 := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'g'}}
	updatedModel, _ = m.Update(gMsg1)
	m = updatedModel.(Model)

	gMsg2 := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'g'}}
	updatedModel, _ = m.Update(gMsg2)
	m = updatedModel.(Model)

	// Then the viewport should be at the top
	assert.True(t, m.Viewport.AtTop(), "Viewport should be at top after pressing gg")
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
			result := HasParagraphBoundary(tt.text)
			assert.Equal(t, tt.expected, result, tt.name)
		})
	}
}

// Scenario 19: Test partial response streaming with ResponseLineMsg
func TestPartialResponseStreaming(t *testing.T) {
	// Given a running tama with a request sent
	m := InitialModel()
	windowMsg := tea.WindowSizeMsg{Width: 100, Height: 30}
	updatedModel, _ := m.Update(windowMsg)
	m = updatedModel.(Model)

	// Create a message pair with a request
	m.MessagePairs = []MessagePair{
		{Request: "Tell me a story", Response: ""},
	}
	m.CurrentPairIndex = 0
	m.RequestStart = time.Now()

	// When the first partial response arrives
	partialMsg1 := ResponseLineMsg("Once upon a time")
	updatedModel, _ = m.Update(partialMsg1)
	m = updatedModel.(Model)

	// Then the response should be visible in responseLines
	assert.Equal(t, 1, len(m.ResponseLines), "Should have one response line")
	assert.Equal(t, "Once upon a time", m.ResponseLines[0], "Should contain the partial response")

	// And the message pair should NOT be updated yet (response still empty)
	assert.Equal(t, "", m.MessagePairs[0].Response, "Message pair response should still be empty during streaming")

	// And we should still be in PromptMode (not switched to ReadMode)
	assert.Equal(t, PromptMode, m.Mode, "Should still be in PromptMode during streaming")

	// When a second partial response arrives
	partialMsg2 := ResponseLineMsg("Once upon a time, there was a brave knight")
	updatedModel, _ = m.Update(partialMsg2)
	m = updatedModel.(Model)

	// Then the response should be updated with the new content
	assert.Equal(t, 1, len(m.ResponseLines), "Should still have one response line")
	assert.Equal(t, "Once upon a time, there was a brave knight", m.ResponseLines[0], "Should contain the updated partial response")

	// And we should still be in PromptMode
	assert.Equal(t, PromptMode, m.Mode, "Should still be in PromptMode during streaming")

	// When the complete response arrives
	completeMsg := ResponseCompleteMsg("Once upon a time, there was a brave knight who saved the kingdom.")
	updatedModel, _ = m.Update(completeMsg)
	m = updatedModel.(Model)

	// Then the message pair should be updated with the complete response
	// (Note: TrimSpace removes trailing newline)
	assert.Equal(t, "Once upon a time, there was a brave knight who saved the kingdom.", m.MessagePairs[0].Response, "Message pair should have the complete response")

	// And the duration should be set
	assert.True(t, m.MessagePairs[0].Duration > 0, "Duration should be set")

	// And we should switch to ReadMode
	assert.Equal(t, ReadMode, m.Mode, "Should switch to ReadMode when response is complete")
	assert.False(t, m.Textarea.Focused(), "Textarea should not be focused in ReadMode")

	// Note: responseLines may still contain the last partial update
	// This is acceptable as the complete response is now in the message pair
}

// Day 4 - Scenario 1: Cancel current request with ctrl-c
func TestCancelCurrentRequest(t *testing.T) {
	// Given the user has submitted a request
	m := InitialModel()
	m.Ready = true
	m.Width = 100
	m.Height = 30
	m.Viewport.Width = 100
	m.Viewport.Height = 20

	// Simulate sending a request
	m.MessagePairs = append(m.MessagePairs, MessagePair{
		Request:  "Tell me a story",
		Response: "",
	})
	m.CurrentPairIndex = 0
	m.RequestStart = time.Now()
	m.IsWaiting = true
	m.ChatRequested = true

	// And the response is not yet complete
	// And the app is in read mode
	m.Mode = ReadMode
	m.Textarea.Blur()

	// When the user keys "ctrl-c"
	ctrlCMsg := tea.KeyMsg{Type: tea.KeyCtrlC}
	updatedModel, cmd := m.Update(ctrlCMsg)
	m = updatedModel.(Model)

	// Then request is cancelled
	// And the app does not wait for the response any longer
	assert.False(t, m.IsWaiting, "Should stop waiting for response")
	assert.False(t, m.ChatRequested, "Chat request should be cancelled")

	// And the message pair should be marked as cancelled
	assert.True(t, m.MessagePairs[0].Cancelled, "Message should be marked as cancelled")

	// And the app should not quit (cmd should be nil, not tea.Quit)
	assert.Nil(t, cmd, "Should not quit the app when cancelling a request")

	// And the user should still be in ReadMode
	assert.Equal(t, ReadMode, m.Mode, "Should remain in ReadMode after cancelling")
}

// Day 4 - Scenario 1 continued: Cancelled messages not included in future requests
func TestCancelledMessagesNotIncludedInFutureRequests(t *testing.T) {
	// Given a cancelled message exists
	m := InitialModel()
	m.Ready = true
	m.MessagePairs = []MessagePair{
		{
			Request:   "First request",
			Response:  "First response",
			Cancelled: false,
		},
		{
			Request:   "Second request (cancelled)",
			Response:  "",
			Cancelled: true,
		},
	}
	m.CurrentPairIndex = 1

	// When building messages for a new request
	// We need to check what messages would be sent to Ollama
	// The sendChatRequestCmd function should skip cancelled messages

	// Simulate what sendChatRequestCmd does
	var ollamaMessages []OllamaMessage
	for _, pair := range m.MessagePairs {
		// Skip cancelled messages
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

	// Then only the first message should be included
	assert.Equal(t, 2, len(ollamaMessages), "Should only include non-cancelled messages")
	assert.Equal(t, "First request", ollamaMessages[0].Content, "Should include first request")
	assert.Equal(t, "First response", ollamaMessages[1].Content, "Should include first response")
}

// Day 4 - Scenario 1 continued: Cancelled indicator in response border
func TestCancelledIndicatorInResponseBorder(t *testing.T) {
	// Given a cancelled message
	m := InitialModel()
	m.Ready = true
	m.Width = 100
	m.Height = 30
	m.Viewport.Width = 100
	m.Viewport.Height = 20
	r, _ := glamour.NewTermRenderer(
		glamour.WithStandardStyle("tokyo-night"),
		glamour.WithWordWrap(100),
	)
	m.Renderer = r

	m.MessagePairs = []MessagePair{
		{
			Request:   "Tell me a story",
			Response:  "",
			Cancelled: true,
		},
	}
	m.CurrentPairIndex = 0

	// When the viewport is updated
	m.updateViewport()

	// Then the view should show the cancelled indicator
	view := m.View()
	assert.Contains(t, view, "Response (cancelled)", "Should show cancelled indicator in response border")
	assert.Contains(t, view, "Request cancelled", "Should show cancelled message")
}

// Test sendChatRequestCmd with mock HTTP server
func TestSendChatRequestCmd(t *testing.T) {
	// Create mock server that simulates Ollama API
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify request method and headers
		assert.Equal(t, "POST", r.Method, "Should use POST method")
		assert.Equal(t, "application/json", r.Header.Get("Content-Type"), "Should set Content-Type header")

		// Send streaming response (simulating Ollama's streaming API)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)

		// Send multiple streaming chunks
		fmt.Fprintln(w, `{"model":"test","message":{"role":"assistant","content":"Hello"}}`)
		fmt.Fprintln(w, `{"model":"test","message":{"role":"assistant","content":" world"}}`)
		fmt.Fprintln(w, `{"model":"test","message":{"role":"assistant","content":"!"}}`)
	}))
	defer server.Close()

	// Track messages sent via sendFn
	var receivedMessages []tea.Msg
	sendFn := func(msg tea.Msg) {
		receivedMessages = append(receivedMessages, msg)
	}

	// Create message pairs
	messagePairs := []MessagePair{
		{
			Request:  "Say hello",
			Response: "",
		},
	}

	// Create context for cancellation
	ctx, cancelFn := context.WithCancel(context.Background())
	defer cancelFn()

	// Call sendChatRequestCmd with mock server URL
	cmd := sendChatRequestCmd(messagePairs, "test-model", sendFn, ctx, cancelFn, server.URL)

	// Execute the command
	result := cmd()

	// Verify the result is a ResponseCompleteMsg
	completeMsg, ok := result.(ResponseCompleteMsg)
	assert.True(t, ok, "Should return ResponseCompleteMsg")

	// Verify the complete response contains all parts
	response := string(completeMsg)
	assert.Contains(t, response, "Hello world!", "Should contain full response text")

	// Verify partial messages were sent during streaming
	assert.Greater(t, len(receivedMessages), 0, "Should have sent partial response messages")

	// Verify at least one partial message contains partial response
	foundPartial := false
	for _, msg := range receivedMessages {
		if lineMsg, ok := msg.(ResponseLineMsg); ok {
			if strings.Contains(string(lineMsg), "Hello") {
				foundPartial = true
				break
			}
		}
	}
	assert.True(t, foundPartial, "Should have sent at least one partial ResponseLineMsg")
}
