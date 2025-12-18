package main

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"

	"github.com/charmbracelet/glamour"
)

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

const (
	ollamaURL    = "http://localhost:11434/api/chat"
	defaultModel = "gpt-oss:20b"
)

func main() {
	fmt.Println("Ollama Chat REPL")
	fmt.Println("Type your message and press Enter. Type 'exit' or 'quit' to exit.")
	fmt.Println("Type 'clear' to clear conversation history.")
	fmt.Println(strings.Repeat("-", 50))

	// Initialize markdown renderer
	r, err := glamour.NewTermRenderer(
		glamour.WithStandardStyle("tokyo-night"),
		glamour.WithWordWrap(100),
	)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error creating markdown renderer: %v\n", err)
		os.Exit(1)
	}

	// Conversation history
	var messages []Message

	// REPL
	scanner := bufio.NewScanner(os.Stdin)
	for {
		fmt.Print("\n> ")
		if !scanner.Scan() {
			break
		}

		input := strings.TrimSpace(scanner.Text())
		if input == "" {
			continue
		}

		// Handle commands
		switch input {
		case "exit", "quit":
			fmt.Println("Goodbye!")
			return
		case "clear":
			messages = []Message{}
			fmt.Println("Conversation history cleared.")
			continue
		}

		// Add user message to history
		messages = append(messages, Message{
			Role:    "user",
			Content: input,
		})

		// Send request to Ollama
		response, err := sendChatRequest(messages)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			// Remove the last user message if request failed
			messages = messages[:len(messages)-1]
			continue
		}

		// Add assistant response to history
		messages = append(messages, Message{
			Role:    "assistant",
			Content: response,
		})

		// Render markdown response
		rendered, err := r.Render(response)
		if err != nil {
			// If markdown rendering fails, just print the raw response
			fmt.Println("\n" + response)
		} else {
			fmt.Print("\n" + rendered)
		}
	}

	if err := scanner.Err(); err != nil {
		fmt.Fprintf(os.Stderr, "Error reading input: %v\n", err)
		os.Exit(1)
	}
}

func sendChatRequest(messages []Message) (string, error) {
	// Create request body
	reqBody := ChatRequest{
		Model:    defaultModel,
		Messages: messages,
		Stream:   false,
	}

	jsonData, err := json.Marshal(reqBody)
	if err != nil {
		return "", fmt.Errorf("failed to marshal request: %w", err)
	}

	// Send HTTP request
	resp, err := http.Post(ollamaURL, "application/json", bytes.NewBuffer(jsonData))
	if err != nil {
		return "", fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("request failed with status %d: %s", resp.StatusCode, string(body))
	}

	// Parse response
	var chatResp ChatResponse
	if err := json.NewDecoder(resp.Body).Decode(&chatResp); err != nil {
		return "", fmt.Errorf("failed to decode response: %w", err)
	}

	return chatResp.Message.Content, nil
}
