package a2aclient

import (
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
)

func TestNew(t *testing.T) {
	os.Setenv("A2A_SERVER_PORT", "1234")
	client, err := New()
	if err != nil {
		t.Fatalf("New() failed: %v", err)
	}
	if client.baseURL != "http://localhost:1234" {
		t.Errorf("Expected baseURL to be http://localhost:1234, got %s", client.baseURL)
	}
}

func TestSendPrompt(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{"jsonrpc":"2.0","id":1,"result":{"kind":"message","message":{"role":"agent","parts":[{"kind":"text","text":"test response"}]}}}`))
	}))
	defer server.Close()

	client := &Client{baseURL: server.URL, httpClient: server.Client()}
	response, err := client.SendPrompt("test prompt")
	if err != nil {
		t.Fatalf("SendPrompt() failed: %v", err)
	}
	if response != "test response" {
		t.Errorf("Expected 'test response', got '%s'", response)
	}
}

func TestSendPromptError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()

	client := &Client{baseURL: server.URL, httpClient: server.Client()}
	_, err := client.SendPrompt("test prompt")
	if err == nil {
		t.Fatal("Expected an error, but got nil")
	}
}
