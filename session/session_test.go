package session

import (
	"gemini-srv/internal/a2aclient"
	"gemini-srv/internal/stats"
	"os"
	"sync"
	"testing"
)

type mockA2AClient struct{}

func (c *mockA2AClient) SendPrompt(prompt string) (string, error) {
	return "mock response", nil
}

func (c *mockA2AClient) SendPromptAsTask(prompt string) (string, error) {
	return "mock-task-id", nil
}

func (c *mockA2AClient) SendPromptStream(prompt string, eventChan chan<- a2aclient.StreamEvent) error {
	defer close(eventChan)
	eventChan <- a2aclient.StreamEvent{Kind: "text", Text: "mock response"}
	return nil
}

var _ a2aclient.A2AClient = &mockA2AClient{}

const testDataBaseDir = "test_session_data_"

func setup(t *testing.T) string {
	baseDir := testDataBaseDir + t.Name()
	if err := os.MkdirAll(baseDir, 0755); err != nil {
		t.Fatalf("Failed to create test base directory: %v", err)
	}
	return baseDir
}

func teardown(t *testing.T) {
	if err := os.RemoveAll(testDataBaseDir + t.Name()); err != nil {
		t.Fatalf("Failed to clean up test directory: %v", err)
	}
}

func TestSessionFileManagement(t *testing.T) {
	baseDir := setup(t)
	defer teardown(t)

	statsManager := stats.New()
	manager, err := NewManager(baseDir, nil, statsManager)
	if err != nil {
		t.Fatalf("NewManager failed: %v", err)
	}

	id := "test-session"
	_, err = manager.CreateSession(id, "/tmp")
	if err != nil {
		t.Fatalf("CreateSession failed: %v", err)
	}

	sessions, err := manager.ListConversations()
	if err != nil {
		t.Fatalf("ListConversations failed: %v", err)
	}
	if len(sessions) != 1 || sessions[0].ID != id {
		t.Errorf("Expected session list to contain only '%s'", id)
	}

	session, err := manager.AcquireSession(id)
	if err != nil {
		t.Fatalf("AcquireSession failed: %v", err)
	}
	if session.ID != id {
		t.Errorf("Acquired session has incorrect ID")
	}

	err = manager.DeleteSession(id)
	if err != nil {
		t.Fatalf("DeleteSession failed: %v", err)
	}
	sessions, _ = manager.ListConversations()
	if len(sessions) != 0 {
		t.Errorf("Expected empty session list after deletion")
	}
}

func TestGenerateNameFromPrompt(t *testing.T) {
	prompt := "hello world this is a test"
	name := generateNameFromPrompt(prompt)
	if name != "hello world this is a" {
		t.Errorf("Expected 'hello world this is a', got '%s'", name)
	}

	prompt = "short"
	name = generateNameFromPrompt(prompt)
	if name != "short" {
		t.Errorf("Expected 'short', got '%s'", name)
	}

	prompt = "a very long prompt that should be truncated to fifty characters or less to be a good name"
	name = generateNameFromPrompt(prompt)
	if len(name) > 50 {
		t.Errorf("Expected name to be 50 characters or less, got %d", len(name))
	}
}

func TestRunPromptAndLoad(t *testing.T) {
	baseDir := setup(t)
	defer teardown(t)

	statsManager := stats.New()
	manager, err := NewManager(baseDir, &mockA2AClient{}, statsManager)
	if err != nil {
		t.Fatalf("NewManager failed: %v", err)
	}

	id := "test-session"
	session, err := manager.CreateSession(id, "/tmp")
	if err != nil {
		t.Fatalf("CreateSession failed: %v", err)
	}

	prompt := "test prompt"
	response, err := manager.RunPrompt(session, prompt)
	if err != nil {
		t.Fatalf("RunPrompt failed: %v", err)
	}
	if response != "mock response" {
		t.Errorf("Expected 'mock response', got '%s'", response)
	}
	if session.History[0] != "User: "+prompt {
		t.Errorf("Expected user prompt in history, got '%s'", session.History[0])
	}
	if session.History[1] != "Gemini: mock response" {
		t.Errorf("Expected gemini response in history, got '%s'", session.History[1])
	}
	if session.Name != "test prompt" {
		t.Errorf("Expected session name to be 'test prompt', got '%s'", session.Name)
	}

	// Clear the session from memory to force a load from disk
	manager.sessions = make(map[string]*Session)

	loadedSession, err := manager.AcquireSession(id)
	if err != nil {
		t.Fatalf("AcquireSession failed: %v", err)
	}
	if loadedSession.ID != id {
		t.Errorf("Acquired session has incorrect ID")
	}
	if loadedSession.History[0] != "User: "+prompt {
		t.Errorf("Expected user prompt in history, got '%s'", loadedSession.History[0])
	}
}

func TestRunPromptAsTask(t *testing.T) {
	baseDir := setup(t)
	defer teardown(t)

	statsManager := stats.New()
	manager, err := NewManager(baseDir, &mockA2AClient{}, statsManager)
	if err != nil {
		t.Fatalf("NewManager failed: %v", err)
	}

	id := "test-session"
	session, err := manager.CreateSession(id, "/tmp")
	if err != nil {
		t.Fatalf("CreateSession failed: %v", err)
	}

	prompt := "test prompt"
	taskID, err := manager.RunPromptAsTask(session, prompt)
	if err != nil {
		t.Fatalf("RunPromptAsTask failed: %v", err)
	}
	if taskID != "mock-task-id" {
		t.Errorf("Expected 'mock-task-id', got '%s'", taskID)
	}
	if session.History[0] != "User: "+prompt {
		t.Errorf("Expected user prompt in history, got '%s'", session.History[0])
	}
	if session.History[1] != "Gemini: (task mock-task-id)" {
		t.Errorf("Expected gemini response in history, got '%s'", session.History[1])
	}
}

func TestRunPromptStream(t *testing.T) {
	baseDir := setup(t)
	defer teardown(t)

	statsManager := stats.New()
	manager, err := NewManager(baseDir, &mockA2AClient{}, statsManager)
	if err != nil {
		t.Fatalf("NewManager failed: %v", err)
	}

	id := "test-session"
	session, err := manager.CreateSession(id, "/tmp")
	if err != nil {
		t.Fatalf("CreateSession failed: %v", err)
	}

	prompt := "test prompt"
	eventChan := make(chan a2aclient.StreamEvent)

	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		err := manager.RunPromptStream(session, prompt, eventChan)
		if err != nil {
			t.Errorf("RunPromptStream failed: %v", err)
		}
	}()

	var events []a2aclient.StreamEvent
	for event := range eventChan {
		events = append(events, event)
	}

	wg.Wait()

	if len(events) != 1 {
		t.Fatalf("Expected 1 event, got %d", len(events))
	}
	if events[0].Kind != "text" || events[0].Text != "mock response" {
		t.Errorf("unexpected event received: %+v", events[0])
	}

	if session.History[0] != "User: "+prompt {
		t.Errorf("Expected user prompt in history, got '%s'", session.History[0])
	}
	if session.History[1] != "Gemini: mock response" {
		t.Errorf("Expected gemini response in history, got '%s'", session.History[1])
	}
}
