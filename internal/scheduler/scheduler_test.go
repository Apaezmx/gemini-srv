package scheduler

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

const testDataBaseDir = "test_scheduler_data_"

func setupTasks(t *testing.T) string {
	baseDir := testDataBaseDir + t.Name()
	tasksDir := filepath.Join(baseDir, "data/tasks")
	if err := os.MkdirAll(tasksDir, 0755); err != nil {
		t.Fatalf("Failed to create test tasks directory: %v", err)
	}
	return baseDir
}

func teardownTasks(t *testing.T) {
	if err := os.RemoveAll(testDataBaseDir + t.Name()); err != nil {
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

func TestRunTask(t *testing.T) {
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
	manager.cron.Stop()

	task, err := manager.parseTask(taskFile)
	if err != nil {
		t.Fatalf("parseTask failed: %v", err)
	}

	manager.runTask(task)

	// Check that the output file was created
	taskOutputDir := filepath.Join(baseDir, "data/task_outputs", "test_task")
	files, err := os.ReadDir(taskOutputDir)
	if err != nil {
		t.Fatalf("Failed to read task output directory: %v", err)
	}
	if len(files) != 1 {
		t.Errorf("Expected 1 output file, got %d", len(files))
	}
}

func TestCleanup(t *testing.T) {
	baseDir := setupTasks(t)
	defer teardownTasks(t)

	manager, err := NewManager(baseDir)
	if err != nil {
		t.Fatalf("NewManager failed during test: %v", err)
	}
	manager.cron.Stop()

	// Create a fake old output file
	taskOutputDir := filepath.Join(baseDir, "data/task_outputs", "test_task")
	if err := os.MkdirAll(taskOutputDir, 0755); err != nil {
		t.Fatalf("Failed to create test task output directory: %v", err)
	}
	oldFile := filepath.Join(taskOutputDir, "old.log")
	if err := os.WriteFile(oldFile, []byte("old"), 0644); err != nil {
		t.Fatalf("Failed to write old file: %v", err)
	}
	twoDaysAgo := time.Now().Add(-48 * time.Hour)
	if err := os.Chtimes(oldFile, twoDaysAgo, twoDaysAgo); err != nil {
		t.Fatalf("Failed to change file modification time: %v", err)
	}

	manager.cleanupOldOutputs()

	files, err := os.ReadDir(taskOutputDir)
	if err != nil {
		t.Fatalf("Failed to read task output directory: %v", err)
	}
	if len(files) != 0 {
		t.Errorf("Expected 0 output files after cleanup, got %d", len(files))
	}
}

func TestFailingTask(t *testing.T) {
	baseDir := setupTasks(t)
	defer teardownTasks(t)

	content := `
name = "Failing Task"
schedule = "* * * * *"
data_command = "exit 1"
prompt = "The data is: {{.Input}}"
`
	tasksDir := filepath.Join(baseDir, "data/tasks")
	taskFile := filepath.Join(tasksDir, "failing_task.toml")
	if err := os.WriteFile(taskFile, []byte(content), 0644); err != nil {
		t.Fatalf("Failed to write test task file: %v", err)
	}

	manager, err := NewManager(baseDir)
	if err != nil {
		t.Fatalf("NewManager failed during test: %v", err)
	}
	manager.cron.Stop()

	task, err := manager.parseTask(taskFile)
	if err != nil {
		t.Fatalf("parseTask failed: %v", err)
	}

	manager.runTask(task)

	// Check that no output file was created
	taskOutputDir := filepath.Join(baseDir, "data/task_outputs", "failing_task")
	_, err = os.ReadDir(taskOutputDir)
	if !os.IsNotExist(err) {
		t.Errorf("Expected task output directory to not exist, but it does")
	}
}