package a2aclient

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"

	"github.com/google/uuid"
)

type StreamEvent struct {
	Kind string          `json:"kind"`
	Text string          `json:"text"`
	Data StreamEventData `json:"data"`
}

type StreamEventData struct {
	Subject     string `json:"subject"`
	Description string `json:"description"`
}

type A2AClient interface {
	SendPrompt(contextID, prompt string) (string, error)
	SendPromptAsTask(contextID, prompt string) (string, error)
	SendPromptStream(contextID, taskID, prompt string, eventChan chan<- StreamEvent) (string, string, error)
}

type Client struct {
	baseURL    string
	httpClient *http.Client
}

// New creates a new a2a-server client.
func New() (*Client, error) {
	port := os.Getenv("A2A_SERVER_PORT")
	if port == "" {
		return nil, fmt.Errorf("A2A_SERVER_PORT environment variable not set")
	}
	return &Client{
		baseURL:    fmt.Sprintf("http://localhost:%s", port),
		httpClient: &http.Client{},
	}, nil
}

// SendPrompt sends a prompt to the a2a-server.
func (c *Client) SendPrompt(contextID, prompt string) (string, error) {
	messageID := uuid.New().String()

	params := map[string]interface{}{
		"message": map[string]interface{}{
			"kind":      "message",
			"role":      "user",
			"messageId": messageID,
			"parts": []map[string]string{
				{"kind": "text", "text": prompt},
			},
		},
	}

	if contextID != "" {
		params["contextId"] = contextID
	}

	requestPayload := map[string]interface{}{
		"jsonrpc": "2.0",
		"id":      1,
		"method":  "message/send",
		"params":  params,
	}

	reqBody, err := json.Marshal(requestPayload)
	if err != nil {
		return "", err
	}

	resp, err := c.httpClient.Post(c.baseURL, "application/json", bytes.NewBuffer(reqBody))
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		responseBytes, _ := io.ReadAll(resp.Body)
		fmt.Printf("a2a-server returned status: %d\n", resp.StatusCode)
		fmt.Printf("Response body: %s\n", string(responseBytes))
		fmt.Printf("Request body: %s\n", reqBody)
		return "", fmt.Errorf("a2a-server returned non-200 status: %d", resp.StatusCode)
	}

	var jsonRpcResponse struct {
		Jsonrpc string `json:"jsonrpc"`
		ID      int    `json:"id"`
		Result  struct {
			Kind    string `json:"kind"`
			History []struct {
				Role  string `json:"role"`
				Parts []struct {
					Kind string `json:"kind"`
					Text string `json:"text"`
				} `json:"parts"`
			} `json:"history"`
			Message struct {
				Role  string `json:"role"`
				Parts []struct {
					Kind string `json:"kind"`
					Text string `json:"text"`
				} `json:"parts"`
			} `json:"message"`
		} `json:"result"`
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}

	if err := json.Unmarshal(body, &jsonRpcResponse); err != nil {
		return "", err
	}

	out, err := json.Marshal(jsonRpcResponse)
	if err != nil {
		return "", err
	}
	fmt.Println(string(out))

	if jsonRpcResponse.Result.Kind == "task" {
		var responseText strings.Builder
		// Iterate through the history to find the last agent message with a text part
		for _, msg := range jsonRpcResponse.Result.History {
			if msg.Role == "agent" {
				for _, part := range msg.Parts {
					if part.Kind == "text" && part.Text != "" {
						if _, err := responseText.WriteString(part.Text); err != nil {
							return "", fmt.Errorf("error writing to responseText: %v", err)
						}
					}
				}
			}
		}
		fmt.Printf("responseText: %s\n", responseText.String())
		return responseText.String(), nil
	} else if jsonRpcResponse.Result.Kind == "message" {
		if jsonRpcResponse.Result.Message.Role == "agent" {
			var responseText strings.Builder
			for _, part := range jsonRpcResponse.Result.Message.Parts {
				if part.Kind == "text" && part.Text != "" {
					responseText.WriteString(part.Text)
				}
			}
			return responseText.String(), nil
		}
	}

	return "", fmt.Errorf("no response text found in a2a-server response")
}

// SendPromptAsTask sends a prompt to the a2a-server and creates a new task.
func (c *Client) SendPromptAsTask(contextID, prompt string) (string, error) {
	messageID := uuid.New().String()

	params := map[string]interface{}{
		"message": map[string]interface{}{
			"kind":      "message",
			"role":      "user",
			"messageId": messageID,
			"parts": []map[string]string{
				{"kind": "text", "text": prompt},
			},
		},
		"configuration": map[string]interface{}{
			"blocking": false,
		},
	}

	if contextID != "" {
		params["contextId"] = contextID
	}

	requestPayload := map[string]interface{}{
		"jsonrpc": "2.0",
		"id":      1,
		"method":  "message/send",
		"params":  params,
	}

	reqBody, err := json.Marshal(requestPayload)
	if err != nil {
		return "", err
	}

	resp, err := c.httpClient.Post(c.baseURL, "application/json", bytes.NewBuffer(reqBody))
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		responseBytes, _ := io.ReadAll(resp.Body)
		fmt.Printf("a2a-server returned status: %d\n", resp.StatusCode)
		fmt.Printf("Response body: %s\n", string(responseBytes))
		fmt.Printf("Request body: %s\n", reqBody)
		return "", fmt.Errorf("a2a-server returned non-200 status: %d", resp.StatusCode)
	}

	var jsonRpcResponse struct {
		Jsonrpc string `json:"jsonrpc"`
		ID      int    `json:"id"`
		Result  struct {
			ID   string `json:"id"`
			Kind string `json:"kind"`
		} `json:"result"`
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}

	if err := json.Unmarshal(body, &jsonRpcResponse); err != nil {
		return "", err
	}

	if jsonRpcResponse.Result.Kind != "task" {
		return "", fmt.Errorf("expected a task object, but got %s", jsonRpcResponse.Result.Kind)
	}

	return jsonRpcResponse.Result.ID, nil
}

// SendPromptStream sends a prompt to the a2a-server and streams the response.
func (c *Client) SendPromptStream(contextID, taskID, prompt string, eventChan chan<- StreamEvent) (string, string, error) {
	messageID := uuid.New().String()

	params := map[string]interface{}{
		"message": map[string]interface{}{
			"kind":      "message",
			"role":      "user",
			"messageId": messageID,
			"parts": []map[string]string{
				{"kind": "text", "text": prompt},
			},
			"contextId": contextID,
			"taskId":    taskID,
		},
	}

	requestPayload := map[string]interface{}{
		"jsonrpc": "2.0",
		"id":      1,
		"method":  "message/stream",
		"params":  params,
	}

	reqBody, err := json.Marshal(requestPayload)
	if err != nil {
		return "", "", err
	}
	fmt.Printf("Sending request to a2a-server: %s\n", string(reqBody))

	req, err := http.NewRequest("POST", c.baseURL, bytes.NewBuffer(reqBody))
	if err != nil {
		return "", "", err
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "text/event-stream")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return "", "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", "", fmt.Errorf("a2a-server returned non-200 status: %d", resp.StatusCode)
	}

	var cID, tID string
	scanner := bufio.NewScanner(resp.Body)
	for scanner.Scan() {
		line := scanner.Text()
		if strings.HasPrefix(line, "data:") {
			data := strings.TrimSpace(strings.TrimPrefix(line, "data:"))
			fmt.Printf("a2a-server event: %s\n", data)
			var sseResponse struct {
				Result json.RawMessage `json:"result"`
			}
			if err := json.Unmarshal([]byte(data), &sseResponse); err != nil {
				fmt.Printf("Error unmarshalling SSE data: %v\n", err)
				continue
			}

			var genericEvent struct {
				Kind      string `json:"kind"`
				ContextID string `json:"contextId"`
				TaskID    string `json:"taskId"`
			}
			if err := json.Unmarshal(sseResponse.Result, &genericEvent); err != nil {
				fmt.Printf("Error unmarshalling generic event: %v\n", err)
				continue
			}

			cID = genericEvent.ContextID
			tID = genericEvent.TaskID

			switch genericEvent.Kind {
			case "message":
				fmt.Println("Received message event")
				var msgEvent struct {
					Parts []struct {
						Text string `json:"text"`
					} `json:"parts"`
				}
				if err := json.Unmarshal(sseResponse.Result, &msgEvent); err == nil {
					for _, part := range msgEvent.Parts {
						eventChan <- StreamEvent{Kind: "text", Text: part.Text}
					}
				}
			case "status-update":
				fmt.Println("Received status_update event")
				var statusEvent struct {
					Status struct {
						Message struct {
							Parts []struct {
								Kind string          `json:"kind"`
								Text string          `json:"text"`
								Data StreamEventData `json:"data"`
							} `json:"parts"`
						} `json:"message"`
					} `json:"status"`
				}
				if err := json.Unmarshal(sseResponse.Result, &statusEvent); err == nil {
					for _, part := range statusEvent.Status.Message.Parts {
						if part.Kind == "text" {
							eventChan <- StreamEvent{Kind: part.Kind, Text: part.Text}
						} else if part.Kind == "data" {
							eventChan <- StreamEvent{Kind: part.Kind, Data: part.Data}
						}
					}
				}
			}
		}
	}

	if err := scanner.Err(); err != nil {
		return "", "", err
	}

	return cID, tID, nil
}
