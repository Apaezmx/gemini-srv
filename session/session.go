package session

import (
	"encoding/json"
	"fmt"
	"gemini-srv/internal/a2aclient"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// Session represents a single user's conversational history.
type Session struct {
	ID               string    `json:"id"`
	History          []string  `json:"history"`
	LastAccess       time.Time `json:"last_access"`
	WorkingDirectory string    `json:"working_directory"`
}

// Manager handles all active sessions.
type Manager struct {
	sessions        map[string]*Session
	mu              sync.Mutex
	sessionDataPath string
	a2aClient       *a2aclient.Client
}

// NewManager creates a new session manager.
func NewManager(baseDir string, client *a2aclient.Client) (*Manager, error) {
	fmt.Println("Creating new session manager...")
	dataPath := filepath.Join(baseDir, "data/conversations")
	if err := os.MkdirAll(dataPath, 0755); err != nil {
		return nil, fmt.Errorf("could not create session data directory: %w", err)
	}
	m := &Manager{
		sessions:        make(map[string]*Session),
		sessionDataPath: dataPath,
		a2aClient:       client,
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
	// The a2a-server manages its own history, but we can prepend our history
	// to the prompt for now to maintain context.
	fullPrompt := strings.Join(s.History, "\n") + "\n" + prompt
	response, err := m.a2aClient.SendPrompt(fullPrompt)

	s.History = append(s.History, "User: "+prompt)
	s.History = append(s.History, "Gemini: "+response)

	if saveErr := s.save(m.sessionDataPath); saveErr != nil {
		return response, fmt.Errorf("original error: %v, failed to save session: %w", err, saveErr)
	}

	return response, err
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

// ListConversations returns the IDs of all persisted conversations.
func (m *Manager) ListConversations() ([]string, error) {
	files, err := os.ReadDir(m.sessionDataPath)
	if err != nil {
		return nil, fmt.Errorf("could not read sessions directory: %w", err)
	}
	var ids []string
	for _, file := range files {
		if !file.IsDir() && strings.HasSuffix(file.Name(), ".json") {
			ids = append(ids, strings.TrimSuffix(file.Name(), ".json"))
		}
	}
	return ids, nil
}