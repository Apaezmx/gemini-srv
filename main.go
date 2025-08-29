package main

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"gemini-srv/internal/a2aclient"
	"gemini-srv/internal/scheduler"
	"gemini-srv/internal/stats"
	"gemini-srv/session"

	"github.com/google/uuid"
	"github.com/gorilla/websocket"
	"github.com/joho/godotenv"
	"github.com/pelletier/go-toml/v2"
)

var (
	sessionManager   *session.Manager
	schedulerManager *scheduler.Manager
	statsManager     *stats.Stats
	executableDir    string
	upgrader         = websocket.Upgrader{
		CheckOrigin: func(r *http.Request) bool {
			return true
		},
	}
)

// isA2AServerRunning checks if the a2a-server is already running on the specified port.
func isA2AServerRunning(port string) bool {
	url := fmt.Sprintf("http://localhost:%s/.well-known/agent-card.json", port)
	client := http.Client{Timeout: 1 * time.Second}
	resp, err := client.Get(url)
	if err != nil {
		if netErr, ok := err.(net.Error); ok && netErr.Timeout() {
			log.Printf("a2a-server timed out on port %s. Assuming not running.", port)
			// Timeout means the server is likely not running or not responding quickly
			return false
		}
		log.Printf("Error checking a2a-server status: %v. Assuming not running.", err)
		// Other errors might indicate connection refused, which also means not running
		return false
	}
	defer resp.Body.Close()
	return resp.StatusCode == http.StatusOK
}

// (Auth and logging middleware remain the same)
func basicAuth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		user := os.Getenv("GEMINI_SRV_USER")
		pass := os.Getenv("GEMINI_SRV_PASS")
		if user == "" || pass == "" {
			http.Error(w, "Server configuration error", http.StatusInternalServerError)
			return
		}
		auth := strings.SplitN(r.Header.Get("Authorization"), " ", 2)
		if len(auth) != 2 || auth[0] != "Basic" {
			w.Header().Set("WWW-Authenticate", `Basic realm="Restricted"`)
			http.Error(w, "authorization failed", http.StatusUnauthorized)
			return
		}

		payload, _ := base64.StdEncoding.DecodeString(auth[1])

		pair := strings.SplitN(string(payload), ":", 2)
		if len(pair) != 2 || pair[0] != user || pair[1] != pass {
			w.Header().Set("WWW-Authenticate", `Basic realm="Restricted"`)
			http.Error(w, "authorization failed", http.StatusUnauthorized)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func httpBasicsLogger(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Cross-Origin-Opener-Policy", "same-origin")
		w.Header().Set("Cross-Origin-Embedder-Policy", "require-corp")
		fmt.Printf("%s %s %s\n", r.RemoteAddr, r.Method, r.URL)
		next.ServeHTTP(w, r)
	})
}

// (API handlers remain the same)
func listConversationsHandler(w http.ResponseWriter, r *http.Request) {
	conversations, err := sessionManager.ListConversations()
	if err != nil {
		http.Error(w, "Failed to list conversations", http.StatusInternalServerError)
		return
	}
	if conversations == nil {
		conversations = make([]session.ConversationInfo, 0)
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(conversations)
}

func createConversationHandler(w http.ResponseWriter, r *http.Request) {
	var reqBody struct {
		ContextPath string `json:"context_path"`
	}
	if err := json.NewDecoder(r.Body).Decode(&reqBody); err != nil && err != io.EOF {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}
	id, err := uuid.NewRandom()
	if err != nil {
		http.Error(w, "Failed to generate session ID", http.StatusInternalServerError)
		return
	}
	sessionID := id.String()
	s, err := sessionManager.CreateSession(sessionID, reqBody.ContextPath)
	if err != nil {
		http.Error(w, "Failed to create session", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(s)
}

func getConversationHandler(w http.ResponseWriter, r *http.Request) {
	id := strings.TrimPrefix(r.URL.Path, "/api/v1/conversations/")
	s, err := sessionManager.AcquireSession(id)
	if err != nil {
		http.Error(w, "Conversation not found", http.StatusNotFound)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(s)
}

func postPromptHandler(w http.ResponseWriter, r *http.Request) {
	id := strings.Split(r.URL.Path, "/")[4]
	s, err := sessionManager.AcquireSession(id)
	if err != nil {
		http.Error(w, "Conversation not found", http.StatusNotFound)
		return
	}
	var reqBody struct {
		Prompt string `json:"prompt"`
		AsTask bool   `json:"as_task"`
	}
	if err := json.NewDecoder(r.Body).Decode(&reqBody); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	if reqBody.AsTask {
		taskID, err := sessionManager.RunPromptAsTask(s, reqBody.Prompt)
		if err != nil {
			fmt.Printf("Error running prompt as task for session %s: %v\n", id, err)
			http.Error(w, "Failed to run prompt as task", http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"task_id": taskID})
	} else {
		response, err := sessionManager.RunPrompt(s, reqBody.Prompt)
		if err != nil {
			fmt.Printf("Error running prompt for session %s: %v\n", id, err)
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"response": response})
	}
}

func postPromptStreamHandler(w http.ResponseWriter, r *http.Request) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Println(err)
		return
	}
	defer conn.Close()

	id := strings.Split(r.URL.Path, "/")[4]
	s, err := sessionManager.AcquireSession(id)
	if err != nil {
		log.Println(err)
		return
	}

	_, p, err := conn.ReadMessage()
	if err != nil {
		log.Println(err)
		return
	}
	prompt := string(p)

	log.Println("Creating event channel in postPromptStreamHandler")
	eventChan := make(chan a2aclient.StreamEvent)

	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		log.Println("Starting goroutine to call RunPromptStream")
		if err := sessionManager.RunPromptStream(s, prompt, eventChan); err != nil {
			log.Printf("Error from RunPromptStream: %v\n", err)
		}
		log.Println("RunPromptStream finished")
		close(eventChan)
	}()

	log.Println("Waiting for events on eventChan...")
	for event := range eventChan {
		log.Printf("Relaying event to websocket: %+v\n", event)
		if err := conn.WriteJSON(event); err != nil {
			log.Printf("Error writing to websocket: %v\n", err)
			return
		}
	}
	log.Println("Event channel closed in postPromptStreamHandler.")
	wg.Wait()
}

func deleteConversationHandler(w http.ResponseWriter, r *http.Request) {
	id := strings.TrimPrefix(r.URL.Path, "/api/v1/conversations/")
	if err := sessionManager.DeleteSession(id); err != nil {
		http.Error(w, "Failed to delete session", http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func listTasksHandler(w http.ResponseWriter, r *http.Request) {
	tasksPath := filepath.Join(executableDir, "data/tasks")
	files, err := os.ReadDir(tasksPath)
	if err != nil {
		http.Error(w, "Failed to read tasks directory", http.StatusInternalServerError)
		return
	}
	tasks := make([]string, 0)
	for _, file := range files {
		if !file.IsDir() && strings.HasSuffix(file.Name(), ".toml") {
			tasks = append(tasks, strings.TrimSuffix(file.Name(), ".toml"))
		}
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(tasks)
}

func getTaskLogsHandler(w http.ResponseWriter, r *http.Request) {
	taskName := strings.Split(r.URL.Path, "/")[4]
	logDir := filepath.Join(executableDir, "data/task_outputs", taskName)
	files, err := os.ReadDir(logDir)
	if err != nil {
		http.Error(w, "Logs not found for task", http.StatusNotFound)
		return
	}
	var logs []string
	for _, file := range files {
		if !file.IsDir() {
			content, err := os.ReadFile(filepath.Join(logDir, file.Name()))
			if err == nil {
				logs = append(logs, string(content))
			}
		}
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(logs)
}

func getTaskDetailsHandler(w http.ResponseWriter, r *http.Request) {
	taskName := strings.TrimPrefix(r.URL.Path, "/api/v1/tasks/")
	taskPath := filepath.Join(executableDir, "data/tasks", taskName+".toml")

	data, err := os.ReadFile(taskPath)
	if err != nil {
		http.Error(w, "Task not found", http.StatusNotFound)
		return
	}

	var task scheduler.Task
	if err := toml.Unmarshal(data, &task); err != nil {
		http.Error(w, "Failed to parse task file", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(task)
}

func deleteTaskHandler(w http.ResponseWriter, r *http.Request) {
	taskName := strings.TrimPrefix(r.URL.Path, "/api/v1/tasks/")
	taskPath := filepath.Join(executableDir, "data/tasks", taskName+".toml")

	if err := os.Remove(taskPath); err != nil {
		http.Error(w, "Failed to delete task", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

func updateTaskHandler(w http.ResponseWriter, r *http.Request) {
	taskName := strings.TrimPrefix(r.URL.Path, "/api/v1/tasks/")
	taskPath := filepath.Join(executableDir, "data/tasks", taskName+".toml")

	var task scheduler.Task
	if err := json.NewDecoder(r.Body).Decode(&task); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	data, err := toml.Marshal(task)
	if err != nil {
		http.Error(w, "Failed to marshal task to TOML", http.StatusInternalServerError)
		return
	}

	if err := os.WriteFile(taskPath, data, 0644); err != nil {
		http.Error(w, "Failed to write task file", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
}

func startA2AServer() {
	startCmd := os.Getenv("A2A_SERVER_START_COMMAND")
	if startCmd == "" {
		log.Fatal("A2A_SERVER_START_COMMAND not set")
	}
	parts := strings.Fields(startCmd)
	cmd := exec.Command(parts[0], parts[1:]...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Start(); err != nil {
		log.Fatalf("Failed to start a2a-server: %v", err)
	}
	go func() {
		cmd.Wait()
		log.Println("a2a-server has exited.")
	}()
}

func main() {
	var err error
	executable, err := os.Executable()
	if err != nil {
		log.Fatal("Could not determine executable path:", err)
	}
	executableDir = filepath.Dir(executable)

	if err := godotenv.Load(filepath.Join(executableDir, ".env")); err != nil {
		log.Println("Warning: .env file not found.")
	}

	a2aServerPort := os.Getenv("A2A_SERVER_PORT")
	if a2aServerPort == "" {
		log.Fatal("A2A_SERVER_PORT environment variable not set")
	}

	if isA2AServerRunning(a2aServerPort) {
		log.Printf("a2a-server already running on port %s. Skipping startup.", a2aServerPort)
	} else {
		log.Printf("Starting a2a-server on port %s...", a2aServerPort)
		// Start the a2a-server in the background
		startA2AServer()
		// Give it a moment to start up before we try to connect
		time.Sleep(3 * time.Second)
	}

	a2aClient, err := a2aclient.New()
	if err != nil {
		log.Fatal("Error creating a2a client:", err)
	}

	statsManager = stats.New()

	sessionManager, err = session.NewManager(executableDir, a2aClient, statsManager)
	if err != nil {
		log.Fatal("Error creating session manager:", err)
	}
	schedulerManager, err = scheduler.NewManager(executableDir)
	if err != nil {
		log.Fatal("Error creating scheduler manager:", err)
	}

	staticDir := filepath.Join(executableDir, "static")
	fs := http.FileServer(http.Dir(staticDir))
	http.Handle("/", fs)
	http.Handle("/static/", http.StripPrefix("/static/", fs))
	http.Handle("/api/", setupRouter())

	port := ":7123"
	fmt.Println("Starting server on ", port)
	if err := http.ListenAndServe(port, nil); err != nil {
		log.Fatal("Error starting server:", err)
	}
}

func setupRouter() http.Handler {
	apiV1 := http.NewServeMux()
	// (API handlers routing remains the same)
	apiV1.HandleFunc("/api/v1/conversations", func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			listConversationsHandler(w, r)
		case http.MethodPost:
			createConversationHandler(w, r)
		default:
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		}
	})
	apiV1.HandleFunc("/api/v1/conversations/", func(w http.ResponseWriter, r *http.Request) {
		if strings.HasSuffix(r.URL.Path, "/prompt") {
			if r.Method == http.MethodPost {
				postPromptHandler(w, r)
			} else {
				http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			}
			return
		}
		if strings.HasSuffix(r.URL.Path, "/prompt/stream") {
			httpBasicsLogger(basicAuth(http.HandlerFunc(postPromptStreamHandler))).ServeHTTP(w, r)
			return
		}
		switch r.Method {
		case http.MethodGet:
			getConversationHandler(w, r)
		case http.MethodDelete:
			deleteConversationHandler(w, r)
		default:
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		}
	})
	apiV1.HandleFunc("/api/v1/tasks", listTasksHandler)
	apiV1.HandleFunc("/api/v1/tasks/", func(w http.ResponseWriter, r *http.Request) {
		if strings.HasSuffix(r.URL.Path, "/logs") {
			getTaskLogsHandler(w, r)
			return
		}
		switch r.Method {
		case http.MethodGet:
			getTaskDetailsHandler(w, r)
		case http.MethodDelete:
			deleteTaskHandler(w, r)
		case http.MethodPut:
			updateTaskHandler(w, r)
		default:
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		}
	})
	apiV1.HandleFunc("/api/v1/model", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"model": "gemini-2.5-pro"})
	})

	apiV1.HandleFunc("/api/v1/stats", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(statsManager.Get())
	})

	return httpBasicsLogger(basicAuth(apiV1))
}
