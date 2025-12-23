package tui

import (
	"fmt"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/charmbracelet/lipgloss"
)

func (m *Model) calculateViewportHeight() int {
	// Layout: TAMA line (1) + viewport + blank (1) + input borders (2) + textarea (dynamic) + status (1)
	fixedHeight := 3
	textareaHeight := m.Textarea.Height()
	inputBorders := 2
	if m.IsWaiting || m.ChatRequested {
		textareaHeight = 1
		inputBorders = 2
	} else if m.Mode == ReadMode {
		textareaHeight = 0
		inputBorders = 0
	}
	return max(m.Height-fixedHeight-textareaHeight-inputBorders, 5)
}

func (m *Model) updateViewport() {
	var content strings.Builder

	// Display only the current message pair
	if len(m.MessagePairs) > 0 && m.CurrentPairIndex < len(m.MessagePairs) {
		pair := m.MessagePairs[m.CurrentPairIndex]

		// Request message with border (straight line)
		requestBorderText := "──── Request "
		remainingWidth := max(m.Viewport.Width-utf8.RuneCountInString(requestBorderText), 0)
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
			var responseBorderText string
			if pair.Cancelled {
				responseBorderText = "──── Response (cancelled) "
			} else {
				durationStr := fmt.Sprintf("%.1fs", pair.Duration.Seconds())
				responseBorderText = fmt.Sprintf("──── Response (%s) ", durationStr)
			}
			remainingWidth := max(m.Viewport.Width-utf8.RuneCountInString(responseBorderText), 0)
			responseBorder := lipgloss.NewStyle().
				Foreground(lipgloss.Color("240")).
				Render(responseBorderText + strings.Repeat("─", remainingWidth))

			content.WriteString(responseBorder)
			content.WriteString("\n")

			// Render response as markdown
			rendered, err := m.Renderer.Render(pair.Response)
			if err != nil {
				content.WriteString(pair.Response)
			} else {
				content.WriteString(rendered)
			}
			content.WriteString("\n")
		} else {
			// Response border without duration (straight line)
			var responseBorderText string
			if pair.Cancelled {
				responseBorderText = "──── Response (cancelled) "
			} else {
				responseBorderText = "──── Response "
			}
			remainingWidth := max(m.Viewport.Width-utf8.RuneCountInString(responseBorderText), 0)
			responseBorder := lipgloss.NewStyle().
				Foreground(lipgloss.Color("240")).
				Render(responseBorderText + strings.Repeat("─", remainingWidth))
			content.WriteString(responseBorder)
			content.WriteString("\n")

			partialResponse := strings.Builder{}
			if pair.Cancelled {
				content.WriteString("Request cancelled\n")
			} else if len(m.ResponseLines) > 0 {
				partialResponse.WriteString("\n")
				partialResponse.WriteString(strings.Join(m.ResponseLines, ""))
				rendered, err := m.Renderer.Render(partialResponse.String())
				if err != nil {
					content.WriteString(partialResponse.String())
				} else {
					content.WriteString(rendered)
				}
			} else {
				content.WriteString("Waiting... \n")
			}
		}
	}

	m.Viewport.SetContent(content.String())
}

func (m Model) View() string {
	if !m.Ready {
		return "Initializing..."
	}

	var b strings.Builder

	// Calculate effective width (at least ContentWidth, or window width if smaller)
	effectiveWidth := min(m.Width, ContentWidth)

	// Calculate left padding to center the content block
	leftPadding := max((m.Width-effectiveWidth)/2, 0)

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
	viewportContent := m.Viewport.View()
	b.WriteString(contentStyle.Render(viewportContent))
	b.WriteString("\n\n")

	// Input area with left padding and minimum width (only in PromptMode)
	// Or show waiting message if waiting for response
	if m.IsWaiting || m.ChatRequested {
		// Show waiting message when response is in progress
		waitingMsg := "Waiting for response, ctrl-c to cancel"
		waitingStyled := lipgloss.NewStyle().
			Width(effectiveWidth).
			Border(lipgloss.NormalBorder(), true, false, true, false).
			BorderForeground(lipgloss.Color("240")).
			Padding(0, 1).
			Render(waitingMsg)
		b.WriteString(contentStyle.Render(waitingStyled))
		b.WriteString("\n")
	} else if m.Mode == PromptMode {
		textareaView := m.Textarea.View()
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
	modelStatus := fmt.Sprintf("Model: %s", m.CurrentModel)
	if !m.ModelIsLoaded {
		modelStatus = fmt.Sprintf("Model: %s (not loaded)", m.CurrentModel)
	}

	// Calculate message count (1-based indexing for display)
	totalPairs := len(m.MessagePairs)
	currentDisplay := 0
	if totalPairs > 0 {
		currentDisplay = m.CurrentPairIndex + 1
	}
	msgCount := fmt.Sprintf("MSG %d/%d", currentDisplay, totalPairs)

	var timerStr string
	if m.LoadingModel {
		elapsed := time.Since(m.LoadingStart)
		timerStr = fmt.Sprintf("⏱  Loading model: %.1fs", elapsed.Seconds())
	} else if m.IsWaiting {
		elapsed := time.Since(m.WaitingStart)
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

	if m.Err != nil {
		errStr := lipgloss.NewStyle().
			Foreground(lipgloss.Color("196")).
			Render(fmt.Sprintf("\nError: %v", m.Err))
		b.WriteString(errStr)
	}

	return b.String()
}
