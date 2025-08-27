package session

import (
	"os"
	"testing"
)

const testDataBaseDir = "test_session_data"

func setup(t *testing.T) string {
	baseDir := testDataBaseDir
	if err := os.MkdirAll(baseDir, 0755); err != nil {
		t.Fatalf("Failed to create test base directory: %v", err)
	}
	return baseDir
}

func teardown(t *testing.T) {
	if err := os.RemoveAll(testDataBaseDir); err != nil {
		t.Fatalf("Failed to clean up test directory: %v", err)
	}
}

func TestSessionFileManagement(t *testing.T) {
	baseDir := setup(t)
	defer teardown(t)

	manager, err := NewManager(baseDir, nil)
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
	if len(sessions) != 1 || sessions[0] != id {
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
