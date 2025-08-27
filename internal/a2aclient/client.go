package a2aclient

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
)

// Client is a simple HTTP client for the a2a-server.
type Client struct {
	baseURL string
}

// New creates a new a2a-server client.
func New() (*Client, error) {
	port := os.Getenv("A2A_SERVER_PORT")
	if port == "" {
		return nil, fmt.Errorf("A2A_SERVER_PORT environment variable not set")
	}
	return &Client{
		baseURL: fmt.Sprintf("http://localhost:%s", port),
	}, nil
}

// SendPrompt sends a prompt to the a2a-server.
func (c *Client) SendPrompt(prompt string) (string, error) {
	reqBody, err := json.Marshal(map[string]string{"prompt": prompt})
	if err != nil {
		return "", err
	}

	resp, err := http.Post(fmt.Sprintf("%s/agent-card.json", c.baseURL), "application/json", bytes.NewBuffer(reqBody))
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("a2a-server returned non-200 status: %d", resp.StatusCode)
	}

	var respBody map[string]string
	if err := json.NewDecoder(resp.Body).Decode(&respBody); err != nil {
		return "", err
	}

	return respBody["response"], nil
}
