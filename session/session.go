package session

import (
	"encoding/json"
	"fmt"
	"gemini-srv/internal/a2aclient"
	"gemini-srv/internal/stats"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// Session represents a single user's conversational history.
type Session struct {
	ID               string    `json:"id"`
	Name             string    `json:"name"`
	History          []string  `json:"history"`
	LastAccess       time.Time `json:"last_access"`
	WorkingDirectory string    `json:"working_directory"`
}

// Manager handles all active sessions.
type Manager struct {
	sessions        map[string]*Session
	mu              sync.Mutex
	sessionDataPath string
	a2aClient       a2aclient.A2AClient
	stats           *stats.Stats
}

// NewManager creates a new session manager.
func NewManager(baseDir string, client a2aclient.A2AClient, stats *stats.Stats) (*Manager, error) {
	fmt.Println("Creating new session manager...")
	dataPath := filepath.Join(baseDir, "data/conversations")
	if err := os.MkdirAll(dataPath, 0755); err != nil {
		return nil, fmt.Errorf("could not create session data directory: %w", err)
	}
	m := &Manager{
		sessions:        make(map[string]*Session),
		sessionDataPath: dataPath,
		a2aClient:       client,
		stats:           stats,
	}
	return m, nil
}

// save persists the session state to a JSON file.
func (s *Session) save(dataPath string) error {
	s.LastAccess = time.Now()
	path := filepath.Join(dataPath, s.ID+".json")
	file, err := os.Create(path)
	if err != nil {
		return fmt.Errorf("could not create session file: %w", err)
	}
	defer file.Close()
	encoder := json.NewEncoder(file)
	encoder.SetIndent("", "  ")
	return encoder.Encode(s)
}

// load retrieves a session from a JSON file.
func (m *Manager) load(sessionID string) (*Session, error) {
	path := filepath.Join(m.sessionDataPath, sessionID+".json")
	file, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("could not open session file: %w", err)
	}
	defer file.Close()
	var s Session
	if err := json.NewDecoder(file).Decode(&s); err != nil {
		return nil, fmt.Errorf("could not decode session file: %w", err)
	}
	return &s, nil
}

// AcquireSession gets a session from the cache or loads it from disk.
func (m *Manager) AcquireSession(sessionID string) (*Session, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if session, ok := m.sessions[sessionID]; ok {
		session.LastAccess = time.Now()
		return session, nil
	}
	session, err := m.load(sessionID)
	if err != nil {
		return nil, err
	}
	m.sessions[sessionID] = session
	return session, nil
}

// CreateSession creates a new session and saves it.
func (m *Manager) CreateSession(sessionID, workingDir string) (*Session, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	session := &Session{
		ID:               sessionID,
		Name:             "New Conversation",
		History:          make([]string, 0),
		LastAccess:       time.Now(),
		WorkingDirectory: workingDir,
	}
	if err := session.save(m.sessionDataPath); err != nil {
		return nil, err
	}
	m.sessions[sessionID] = session
	return session, nil
}

// RunPrompt sends a prompt to the a2a-server.
func (m *Manager) RunPrompt(s *Session, prompt string) (string, error) {
	startTime := time.Now()
	response, err := m.a2aClient.SendPrompt(s.ID, prompt)
	latency := time.Since(startTime)

	m.stats.RecordCall(latency, len(prompt), len(response))

	if len(s.History) == 0 {
		s.Name = generateNameFromPrompt(prompt)
	}

	s.History = append(s.History, "User: "+prompt)
	s.History = append(s.History, "Gemini: "+response)

	if saveErr := s.save(m.sessionDataPath); saveErr != nil {
		return response, fmt.Errorf("original error: %v, failed to save session: %w", err, saveErr)
	}

	return response, err
}

// RunPromptAsTask sends a prompt to the a2a-server and creates a new task.
func (m *Manager) RunPromptAsTask(s *Session, prompt string) (string, error) {
	startTime := time.Now()
	taskID, err := m.a2aClient.SendPromptAsTask(s.ID, prompt)
	latency := time.Since(startTime)

	m.stats.RecordCall(latency, len(prompt), 0)

	if len(s.History) == 0 {
		s.Name = generateNameFromPrompt(prompt)
	}

	s.History = append(s.History, "User: "+prompt)
	s.History = append(s.History, "Gemini: (task "+taskID+")")

	if saveErr := s.save(m.sessionDataPath); saveErr != nil {
		return taskID, fmt.Errorf("original error: %v, failed to save session: %w", err, saveErr)
	}

	return taskID, err
}

// RunPromptStream sends a prompt to the a2a-server and streams the response.
func (m *Manager) RunPromptStream(s *Session, prompt string, eventChan chan<- a2aclient.StreamEvent) error {
	startTime := time.Now()
	var responseText strings.Builder

	internalChan := make(chan a2aclient.StreamEvent)
	var wg sync.WaitGroup
	wg.Add(1)

	go func() {
		defer wg.Done()
		for event := range internalChan {
			if event.Kind == "text" {
				responseText.WriteString(event.Text)
			}
			eventChan <- event
		}
	}()

	err := m.a2aClient.SendPromptStream(s.ID, prompt, internalChan)
	close(internalChan)
	wg.Wait()

	latency := time.Since(startTime)
	m.stats.RecordCall(latency, len(prompt), responseText.Len())

	if len(s.History) == 0 {
		s.Name = generateNameFromPrompt(prompt)
	}

	s.History = append(s.History, "User: "+prompt)
	s.History = append(s.History, "Gemini: "+responseText.String())

	if saveErr := s.save(m.sessionDataPath); saveErr != nil {
		if err != nil {
			return fmt.Errorf("stream error: %v, failed to save session: %w", err, saveErr)
		}
		return fmt.Errorf("failed to save session: %w", saveErr)
	}

	return err
}

// DeleteSession deletes the session file.
func (m *Manager) DeleteSession(sessionID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.sessions, sessionID)
	path := filepath.Join(m.sessionDataPath, sessionID+".json")
	if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("could not delete session file: %w", err)
	}
	fmt.Printf("Deleted session %s\n", sessionID)
	return nil
}

type ConversationInfo struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

// ListConversations returns the IDs and names of all persisted conversations.
func (m *Manager) ListConversations() ([]ConversationInfo, error) {
	files, err := os.ReadDir(m.sessionDataPath)
	if err != nil {
		return nil, fmt.Errorf("could not read sessions directory: %w", err)
	}
	var conversations []ConversationInfo
	for _, file := range files {
		if !file.IsDir() && strings.HasSuffix(file.Name(), ".json") {
			sessionID := strings.TrimSuffix(file.Name(), ".json")
			session, err := m.AcquireSession(sessionID)
			if err != nil {
				// Log the error and skip the conversation
				fmt.Printf("Error loading conversation %s: %v\n", sessionID, err)
				continue
			}
			conversations = append(conversations, ConversationInfo{ID: session.ID, Name: session.Name})
		}
	}
	return conversations, nil
}

func generateNameFromPrompt(prompt string) string {
	words := strings.Fields(prompt)
	if len(words) > 5 {
		words = words[:5]
	}
	name := strings.Join(words, " ")
	if len(name) > 50 {
		name = name[:50]
	}
	return name
}
