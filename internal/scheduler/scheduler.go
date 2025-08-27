package scheduler

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"text/template"
	"time"

	"github.com/pelletier/go-toml/v2"
	"github.com/robfig/cron/v3"
)

var (
	outputTTL = 24 * time.Hour
)

// Task defines the structure of a TOML task definition file.
type Task struct {
	Name        string `toml:"name"`
	Description string `toml:"description"`
	Schedule    string `toml:"schedule"`
	ContextPath string `toml:"context_path"`
	DataCommand string `toml:"data_command"`
	Prompt      string `toml:"prompt"`
}

// Manager handles the scheduling and execution of tasks.
type Manager struct {
	cron           *cron.Cron
	taskDefsPath   string
	taskOutputPath string
}

// NewManager creates and starts a new task scheduler manager.
func NewManager(baseDir string) (*Manager, error) {
	defsPath := filepath.Join(baseDir, "data/tasks")
	outPath := filepath.Join(baseDir, "data/task_outputs")
	if err := os.MkdirAll(defsPath, 0755); err != nil {
		return nil, fmt.Errorf("could not create task defs directory: %w", err)
	}
	if err := os.MkdirAll(outPath, 0755); err != nil {
		return nil, fmt.Errorf("could not create task output directory: %w", err)
	}

	m := &Manager{
		cron:           cron.New(),
		taskDefsPath:   defsPath,
		taskOutputPath: outPath,
	}

	if err := m.loadAndScheduleTasks(); err != nil {
		return nil, err
	}

	_, err := m.cron.AddFunc("@hourly", m.cleanupOldOutputs)
	if err != nil {
		return nil, fmt.Errorf("failed to schedule cleanup job: %w", err)
	}

	m.cron.Start()
	fmt.Println("Scheduler started. Loaded tasks and scheduled hourly cleanup.")
	return m, nil
}

// loadAndScheduleTasks scans the tasks directory and schedules all found tasks.
func (m *Manager) loadAndScheduleTasks() error {
	files, err := os.ReadDir(m.taskDefsPath)
	if err != nil {
		return fmt.Errorf("failed to read task definitions directory: %w", err)
	}

	for _, file := range files {
		if !file.IsDir() && strings.HasSuffix(file.Name(), ".toml") {
			task, err := m.parseTask(filepath.Join(m.taskDefsPath, file.Name()))
			if err != nil {
				fmt.Printf("Warning: Skipping invalid task file %s: %v\n", file.Name(), err)
				continue
			}

			taskToRun := task
			_, err = m.cron.AddFunc(task.Schedule, func() {
				m.runTask(taskToRun)
			})

			if err != nil {
				fmt.Printf("Warning: Skipping invalid schedule for task %s: %v\n", task.Name, err)
				continue
			}
			fmt.Printf("Scheduled task: '%s' with schedule: '%s'\n", task.Name, task.Schedule)
		}
	}
	return nil
}

// parseTask reads and decodes a single TOML task file.
func (m *Manager) parseTask(path string) (*Task, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var task Task
	if err := toml.Unmarshal(data, &task); err != nil {
		return nil, err
	}
	return &task, nil
}

// runTask is the core logic for executing a single task.
func (m *Manager) runTask(t *Task) {
	fmt.Printf("Running task: %s\n", t.Name)

	cmd := exec.Command("bash", "-c", t.DataCommand)
	output, err := cmd.CombinedOutput()
	if err != nil {
		fmt.Printf("Error executing data_command for task '%s': %v\nOutput: %s\n", t.Name, err, string(output))
		return
	}

	inputData := strings.TrimSpace(string(output))
	if inputData == "" {
		fmt.Printf("Task '%s' produced no data. Skipping Gemini call.\n", t.Name)
		return
	}

	promptTemplate, err := template.New("prompt").Parse(t.Prompt)
	if err != nil {
		fmt.Printf("Error parsing prompt template for task '%s': %v\n", t.Name, err)
		return
	}
	var finalPrompt bytes.Buffer
	if err := promptTemplate.Execute(&finalPrompt, map[string]string{"Input": inputData}); err != nil {
		fmt.Printf("Error executing prompt template for task '%s': %v\n", t.Name, err)
		return
	}

	// This is where the a2a client would be used.
	// For now, we will just log the prompt that would be sent.
	fmt.Printf("Task '%s' would send prompt: %s\n", t.Name, finalPrompt.String())

	// We don't have stderr or exit code in this simplified model, so we'll just save the output.
	if err := m.saveOutput(t, "Prompt would be sent, but a2a client is not implemented in scheduler yet."); err != nil {
		fmt.Printf("Error saving output for task '%s': %v\n", t.Name, err)
	}
}

// saveOutput writes the result of a task run to a timestamped file.
func (m *Manager) saveOutput(t *Task, output string) error {
	taskDir := filepath.Join(m.taskOutputPath, strings.ReplaceAll(strings.ToLower(t.Name), " ", "_"))
	if err := os.MkdirAll(taskDir, 0755); err != nil {
		return err
	}

	ts := time.Now().Format("2006-01-02T15-04-05")
	logFile := filepath.Join(taskDir, ts+".log")

	content := fmt.Sprintf(`--- Task Run: %s ---
Timestamp: %s

--- STDOUT ---
%s
`, t.Name, time.Now().Format(time.RFC3339), output)

	return os.WriteFile(logFile, []byte(content), 0644)
}

// cleanupOldOutputs scans the output directory and deletes files older than the TTL.
func (m *Manager) cleanupOldOutputs() {
	fmt.Println("Running hourly cleanup of old task outputs...")
	err := filepath.Walk(m.taskOutputPath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if !info.IsDir() && time.Since(info.ModTime()) > outputTTL {
			fmt.Printf("Deleting old task output: %s\n", path)
			return os.Remove(path)
		}
		return nil
	})
	if err != nil {
		fmt.Printf("Error during task output cleanup: %v\n", err)
	}
}