package main

import (
	"bytes"
	"gemini-srv/internal/a2aclient"
	"gemini-srv/internal/stats"
	"gemini-srv/session"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gorilla/websocket"
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

func TestModelHandler(t *testing.T) {
	os.Setenv("GEMINI_SRV_USER", "test")
	os.Setenv("GEMINI_SRV_PASS", "test")
	executableDir, _ = os.Getwd()
	router := setupRouter()
	req, err := http.NewRequest("GET", "/api/v1/model", nil)
	if err != nil {
		t.Fatal(err)
	}
	req.SetBasicAuth("test", "test")

	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, req)

	if status := rr.Code; status != http.StatusOK {
		t.Errorf("handler returned wrong status code: got %v want %v",
			status, http.StatusOK)
	}

	expected := `{"model":"gemini-2.5-pro"}`
	if strings.TrimSpace(rr.Body.String()) != expected {
		t.Errorf("handler returned unexpected body: got %v want %v",
			rr.Body.String(), expected)
	}
}

func TestStatsHandler(t *testing.T) {
	os.Setenv("GEMINI_SRV_USER", "test")
	os.Setenv("GEMINI_SRV_PASS", "test")
	executableDir, _ = os.Getwd()
	router := setupRouter()
	statsManager = stats.New()
	req, err := http.NewRequest("GET", "/api/v1/stats", nil)
	if err != nil {
		t.Fatal(err)
	}
	req.SetBasicAuth("test", "test")

	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, req)

	if status := rr.Code; status != http.StatusOK {
		t.Errorf("handler returned wrong status code: got %v want %v",
			status, http.StatusOK)
	}

	expected := `{"avg_latency_ms":0,"total_calls":0,"total_chars_in":0,"total_chars_out":0}`
	if strings.TrimSpace(rr.Body.String()) != expected {
		t.Errorf("handler returned unexpected body: got %v want %v",
			rr.Body.String(), expected)
	}
}

func TestListConversationsHandler(t *testing.T) {
	os.Setenv("GEMINI_SRV_USER", "test")
	os.Setenv("GEMINI_SRV_PASS", "test")
	executableDir, _ = os.Getwd()
	testDir := filepath.Join(executableDir, "data/conversations")
	os.RemoveAll(testDir)
	os.MkdirAll(testDir, 0755)
	router := setupRouter()
	sessionManager, _ = session.NewManager(executableDir, &mockA2AClient{}, stats.New())
	req, err := http.NewRequest("GET", "/api/v1/conversations", nil)
	if err != nil {
		t.Fatal(err)
	}
	req.SetBasicAuth("test", "test")

	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, req)

	if status := rr.Code; status != http.StatusOK {
		t.Errorf("handler returned wrong status code: got %v want %v",
			status, http.StatusOK)
	}

	expected := `[]`
	if strings.TrimSpace(rr.Body.String()) != expected {
		t.Errorf("handler returned unexpected body: got %v want %v",
			rr.Body.String(), expected)
	}
}

func TestCreateConversationHandler(t *testing.T) {
	os.Setenv("GEMINI_SRV_USER", "test")
	os.Setenv("GEMINI_SRV_PASS", "test")
	executableDir, _ = os.Getwd()
	testDir := filepath.Join(executableDir, "data/conversations")
	os.RemoveAll(testDir)
	os.MkdirAll(testDir, 0755)
	router := setupRouter()
	sessionManager, _ = session.NewManager(executableDir, &mockA2AClient{}, stats.New())
	req, err := http.NewRequest("POST", "/api/v1/conversations", bytes.NewBuffer([]byte(`{"context_path": ""}`)))
	if err != nil {
		t.Fatal(err)
	}
	req.SetBasicAuth("test", "test")

	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, req)

	if status := rr.Code; status != http.StatusCreated {
		t.Errorf("handler returned wrong status code: got %v want %v",
			status, http.StatusCreated)
	}
}

func TestGetConversationHandler(t *testing.T) {
	os.Setenv("GEMINI_SRV_USER", "test")
	os.Setenv("GEMINI_SRV_PASS", "test")
	executableDir, _ = os.Getwd()
	testDir := filepath.Join(executableDir, "data/conversations")
	os.RemoveAll(testDir)
	os.MkdirAll(testDir, 0755)
	router := setupRouter()
	sessionManager, _ = session.NewManager(executableDir, &mockA2AClient{}, stats.New())
	sessionManager.CreateSession("test-session", "")
	req, err := http.NewRequest("GET", "/api/v1/conversations/test-session", nil)
	if err != nil {
		t.Fatal(err)
	}
	req.SetBasicAuth("test", "test")

	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, req)

	if status := rr.Code; status != http.StatusOK {
		t.Errorf("handler returned wrong status code: got %v want %v",
			status, http.StatusOK)
	}

	expected := `{"id":"test-session","name":"New Conversation","history":[],"last_access":"`
	if !strings.HasPrefix(rr.Body.String(), expected) {
		t.Errorf("handler returned unexpected body: got %v want %v",
			rr.Body.String(), expected)
	}
}

func TestPostPromptHandler(t *testing.T) {
	os.Setenv("GEMINI_SRV_USER", "test")
	os.Setenv("GEMINI_SRV_PASS", "test")
	executableDir, _ = os.Getwd()
	testDir := filepath.Join(executableDir, "data/conversations")
	os.RemoveAll(testDir)
	os.MkdirAll(testDir, 0755)
	router := setupRouter()
	sessionManager, _ = session.NewManager(executableDir, &mockA2AClient{}, stats.New())
	sessionManager.CreateSession("test-session", "")
	req, err := http.NewRequest("POST", "/api/v1/conversations/test-session/prompt", bytes.NewBuffer([]byte(`{"prompt": "test prompt"}`)))
	if err != nil {
		t.Fatal(err)
	}
	req.SetBasicAuth("test", "test")

	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, req)

	if status := rr.Code; status != http.StatusOK {
		t.Errorf("handler returned wrong status code: got %v want %v",
			status, http.StatusOK)
	}

	expected := `{"response":"mock response"}`
	if strings.TrimSpace(rr.Body.String()) != expected {
		t.Errorf("handler returned unexpected body: got %v want %v",
			rr.Body.String(), expected)
	}
}

func TestPostPromptHandlerAsTask(t *testing.T) {
	os.Setenv("GEMINI_SRV_USER", "test")
	os.Setenv("GEMINI_SRV_PASS", "test")
	executableDir, _ = os.Getwd()
	testDir := filepath.Join(executableDir, "data/conversations")
	os.RemoveAll(testDir)
	os.MkdirAll(testDir, 0755)
	router := setupRouter()
	sessionManager, _ = session.NewManager(executableDir, &mockA2AClient{}, stats.New())
	sessionManager.CreateSession("test-session", "")
	req, err := http.NewRequest("POST", "/api/v1/conversations/test-session/prompt", bytes.NewBuffer([]byte(`{"prompt": "test prompt", "as_task": true}`)))
	if err != nil {
		t.Fatal(err)
	}
	req.SetBasicAuth("test", "test")

	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, req)

	if status := rr.Code; status != http.StatusOK {
		t.Errorf("handler returned wrong status code: got %v want %v",
			status, http.StatusOK)
	}

	expected := `{"task_id":"mock-task-id"}`
	if strings.TrimSpace(rr.Body.String()) != expected {
		t.Errorf("handler returned unexpected body: got %v want %v",
			rr.Body.String(), expected)
	}
}

func TestDeleteConversationHandler(t *testing.T) {
	os.Setenv("GEMINI_SRV_USER", "test")
	os.Setenv("GEMINI_SRV_PASS", "test")
	executableDir, _ = os.Getwd()
	testDir := filepath.Join(executableDir, "data/conversations")
	os.RemoveAll(testDir)
	os.MkdirAll(testDir, 0755)
	router := setupRouter()
	sessionManager, _ = session.NewManager(executableDir, &mockA2AClient{}, stats.New())
	sessionManager.CreateSession("test-session", "")
	req, err := http.NewRequest("DELETE", "/api/v1/conversations/test-session", nil)
	if err != nil {
		t.Fatal(err)
	}
	req.SetBasicAuth("test", "test")

	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, req)

	if status := rr.Code; status != http.StatusNoContent {
		t.Errorf("handler returned wrong status code: got %v want %v",
			status, http.StatusNoContent)
	}
}

func TestListTasksHandler(t *testing.T) {
	os.Setenv("GEMINI_SRV_USER", "test")
	os.Setenv("GEMINI_SRV_PASS", "test")
	executableDir, _ = os.Getwd()
	testDir := filepath.Join(executableDir, "data/tasks")
	os.RemoveAll(testDir)
	os.MkdirAll(testDir, 0755)
	router := setupRouter()
	req, err := http.NewRequest("GET", "/api/v1/tasks", nil)
	if err != nil {
		t.Fatal(err)
	}
	req.SetBasicAuth("test", "test")

	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, req)

	if status := rr.Code; status != http.StatusOK {
		t.Errorf("handler returned wrong status code: got %v want %v",
			status, http.StatusOK)
	}

	expected := `[]`
	if strings.TrimSpace(rr.Body.String()) != expected {
		t.Errorf("handler returned unexpected body: got %v want %v",
			rr.Body.String(), expected)
	}
}

func TestGetTaskDetailsHandler(t *testing.T) {
	os.Setenv("GEMINI_SRV_USER", "test")
	os.Setenv("GEMINI_SRV_PASS", "test")
	executableDir, _ = os.Getwd()
	testDir := filepath.Join(executableDir, "data/tasks")
	os.RemoveAll(testDir)
	os.MkdirAll(testDir, 0755)
	taskFile := filepath.Join(testDir, "test-task.toml")
	os.WriteFile(taskFile, []byte(`name = "Test Task"`), 0644)
	router := setupRouter()
	req, err := http.NewRequest("GET", "/api/v1/tasks/test-task", nil)
	if err != nil {
		t.Fatal(err)
	}
	req.SetBasicAuth("test", "test")

	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, req)

	if status := rr.Code; status != http.StatusOK {
		t.Errorf("handler returned wrong status code: got %v want %v",
			status, http.StatusOK)
	}

	expected := `{"Name":"Test Task","Description":"","Schedule":"","ContextPath":"","DataCommand":"","Prompt":""}`
	if strings.TrimSpace(rr.Body.String()) != expected {
		t.Errorf("handler returned unexpected body: got %v want %v",
			rr.Body.String(), expected)
	}
}

func TestDeleteTaskHandler(t *testing.T) {
	os.Setenv("GEMINI_SRV_USER", "test")
	os.Setenv("GEMINI_SRV_PASS", "test")
	executableDir, _ = os.Getwd()
	testDir := filepath.Join(executableDir, "data/tasks")
	os.RemoveAll(testDir)
	os.MkdirAll(testDir, 0755)
	taskFile := filepath.Join(testDir, "test-task.toml")
	os.WriteFile(taskFile, []byte(`name = "Test Task"`), 0644)
	router := setupRouter()
	req, err := http.NewRequest("DELETE", "/api/v1/tasks/test-task", nil)
	if err != nil {
		t.Fatal(err)
	}
	req.SetBasicAuth("test", "test")

	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, req)

	if status := rr.Code; status != http.StatusNoContent {
		t.Errorf("handler returned wrong status code: got %v want %v",
			status, http.StatusNoContent)
	}
}

func TestUpdateTaskHandler(t *testing.T) {
	os.Setenv("GEMINI_SRV_USER", "test")
	os.Setenv("GEMINI_SRV_PASS", "test")
	executableDir, _ = os.Getwd()
	testDir := filepath.Join(executableDir, "data/tasks")
	os.RemoveAll(testDir)
	os.MkdirAll(testDir, 0755)
	taskFile := filepath.Join(testDir, "test-task.toml")
	os.WriteFile(taskFile, []byte(`name = "Test Task"`), 0644)
	router := setupRouter()
	req, err := http.NewRequest("PUT", "/api/v1/tasks/test-task", bytes.NewBuffer([]byte(`{"name":"Test Task","description":"new description"}`)))
	if err != nil {
		t.Fatal(err)
	}
	req.SetBasicAuth("test", "test")

	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, req)

	if status := rr.Code; status != http.StatusOK {
		t.Errorf("handler returned wrong status code: got %v want %v",
			status, http.StatusOK)
	}
}

func TestGetTaskLogsHandler(t *testing.T) {
	os.Setenv("GEMINI_SRV_USER", "test")
	os.Setenv("GEMINI_SRV_PASS", "test")
	executableDir, _ = os.Getwd()
	testDir := filepath.Join(executableDir, "data/task_outputs/test-task")
	os.RemoveAll(testDir)
	os.MkdirAll(testDir, 0755)
	logFile := filepath.Join(testDir, "test.log")
	os.WriteFile(logFile, []byte("test log"), 0644)
	router := setupRouter()
	req, err := http.NewRequest("GET", "/api/v1/tasks/test-task/logs", nil)
	if err != nil {
		t.Fatal(err)
	}
	req.SetBasicAuth("test", "test")

	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, req)

	if status := rr.Code; status != http.StatusOK {
		t.Errorf("handler returned wrong status code: got %v want %v",
			status, http.StatusOK)
	}

	expected := `["test log"]`
	if strings.TrimSpace(rr.Body.String()) != expected {
		t.Errorf("handler returned unexpected body: got %v want %v",
			rr.Body.String(), expected)
	}
}

func TestPostPromptStreamHandler(t *testing.T) {
	os.Setenv("GEMINI_SRV_USER", "test")
	os.Setenv("GEMINI_SRV_PASS", "test")
	executableDir, _ = os.Getwd()
	testDir := filepath.Join(executableDir, "data/conversations")
	os.RemoveAll(testDir)
	os.MkdirAll(testDir, 0755)
	router := setupRouter()
	sessionManager, _ = session.NewManager(executableDir, &mockA2AClient{}, stats.New())
	sessionManager.CreateSession("test-session", "")

	server := httptest.NewServer(router)
	defer server.Close()

	wsURL := "ws" + strings.TrimPrefix(server.URL, "http") + "/api/v1/conversations/test-session/prompt/stream"

	header := http.Header{}
	header.Set("Authorization", "Basic dGVzdDp0ZXN0")

	ws, _, err := websocket.DefaultDialer.Dial(wsURL, header)
	if err != nil {
		t.Fatalf("could not open websocket: %v", err)
	}
	defer ws.Close()

	if err := ws.WriteMessage(websocket.TextMessage, []byte("test prompt")); err != nil {
		t.Fatalf("could not send message over websocket: %v", err)
	}

	var event a2aclient.StreamEvent
	if err := ws.ReadJSON(&event); err != nil {
		t.Fatalf("could not read message from websocket: %v", err)
	}

	if event.Kind != "text" || event.Text != "mock response" {
		t.Errorf("unexpected event received: %+v", event)
	}
}