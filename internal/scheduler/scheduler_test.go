package scheduler

import (
	"os"
	"path/filepath"
	"testing"
)

const testDataBaseDir = "test_scheduler_data"

func setupTasks(t *testing.T) string {
	baseDir := testDataBaseDir
	tasksDir := filepath.Join(baseDir, "data/tasks")
	if err := os.MkdirAll(tasksDir, 0755); err != nil {
		t.Fatalf("Failed to create test tasks directory: %v", err)
	}
	return baseDir
}

func teardownTasks(t *testing.T) {
	if err := os.RemoveAll(testDataBaseDir); err != nil {
		t.Fatalf("Failed to clean up test tasks directory: %v", err)
	}
}

func TestTaskParsing(t *testing.T) {
	baseDir := setupTasks(t)
	defer teardownTasks(t)

	content := `
name = "Test Task"
schedule = "* * * * *"
data_command = "echo 'hello'"
prompt = "The data is: {{.Input}}"
`
	tasksDir := filepath.Join(baseDir, "data/tasks")
	taskFile := filepath.Join(tasksDir, "test_task.toml")
	if err := os.WriteFile(taskFile, []byte(content), 0644); err != nil {
		t.Fatalf("Failed to write test task file: %v", err)
	}

	manager, err := NewManager(baseDir)
	if err != nil {
		t.Fatalf("NewManager failed during test: %v", err)
	}
	manager.cron.Stop() // Stop the cron scheduler as we are only testing parsing

	task, err := manager.parseTask(taskFile)
	if err != nil {
		t.Fatalf("parseTask failed: %v", err)
	}

	if task.Name != "Test Task" {
		t.Errorf("Expected task name 'Test Task', got '%s'", task.Name)
	}
}