document.addEventListener('DOMContentLoaded', () => {
    // --- STATE ---
    let currentConversationId = null;

    // --- CONSTANTS ---
    const funStatements = [
        "Consulting the digital oracle...",
        "Reticulating splines...",
        "Asking the hive mind...",
        "Warming up the neural nets...",
        "Checking my sources (I have many)...",
        "Polishing the response...",
        "Translating from binary...",
        "Summoning the code spirits...",
    ];

    // --- DOM ELEMENTS ---
    const newConvBtn = document.getElementById('new-conv-btn');
    const conversationsList = document.getElementById('conversations-list');
    const tasksList = document.getElementById('tasks-list');
    const welcomeView = document.getElementById('welcome-view');
    const conversationView = document.getElementById('conversation-view');
    const taskView = document.getElementById('task-view');
    const convTitle = document.getElementById('conv-title');
    const deleteConvBtn = document.getElementById('delete-conv-btn');
    const chatHistory = document.getElementById('chat-history');
    const promptTextarea = document.getElementById('prompt-textarea');
    const sendPromptBtn = document.getElementById('send-prompt-btn');
    const taskTitle = document.getElementById('task-title');
    const taskDetailsView = document.getElementById('task-details-view');
    const taskForm = document.getElementById('task-form');
    const deleteTaskBtn = document.getElementById('delete-task-btn');
    const taskLogsView = document.getElementById('task-logs-view');
    const taskLogs = taskLogsView.querySelector('pre');

    const modelInfo = document.getElementById('model-info');
    const statsInfo = document.getElementById('stats-info');

    // --- API FUNCTIONS ---
    const api = {
        getConversations: () => fetch('/api/v1/conversations').then(res => res.json()),
        createConversation: (contextPath = '') => fetch('/api/v1/conversations', {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify({ context_path: contextPath }),
        }).then(res => res.json()),
        getConversation: (id) => fetch(`/api/v1/conversations/${id}`).then(res => res.json()),
        deleteConversation: (id) => fetch(`/api/v1/conversations/${id}`, { method: 'DELETE' }),
        getTasks: () => fetch('/api/v1/tasks').then(res => res.json()),
        getTaskLogs: (taskName) => fetch(`/api/v1/tasks/${taskName}/logs`).then(res => res.json()),
        getTaskDetails: (taskName) => fetch(`/api/v1/tasks/${taskName}`).then(res => res.json()),
        deleteTask: (taskName) => fetch(`/api/v1/tasks/${taskName}`, { method: 'DELETE' }),
        updateTask: (taskName, task) => fetch(`/api/v1/tasks/${taskName}`, {
            method: 'PUT',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify(task),
        }),
        getModel: () => fetch('/api/v1/model').then(res => res.json()),
        getStats: () => fetch('/api/v1/stats').then(res => res.json()),
    };

    // --- RENDER FUNCTIONS ---
    const renderConversations = async () => {
        const conversations = await api.getConversations();
        conversationsList.innerHTML = '';
        conversations.forEach(conv => {
            const li = document.createElement('li');
            li.textContent = conv.name;
            li.dataset.id = conv.id;
            li.addEventListener('click', () => selectConversation(conv.id));
            conversationsList.appendChild(li);
        });
    };

    const renderTasks = async () => {
        const tasks = await api.getTasks();
        tasksList.innerHTML = '';
        tasks.forEach(taskName => {
            const li = document.createElement('li');
            li.textContent = taskName;
            li.dataset.name = taskName;
            li.addEventListener('click', () => selectTask(taskName));
            tasksList.appendChild(li);
        });
    };

    const renderChatHistory = (history) => {
        chatHistory.innerHTML = '';
        history.forEach(line => {
            const parts = line.split(': ');
            const type = parts[0];
            const content = parts.slice(1).join(': ');
            const messageDiv = document.createElement('div');
            messageDiv.className = `message ${type.toLowerCase()}`;
            messageDiv.textContent = content;
            chatHistory.appendChild(messageDiv);
        });
        chatHistory.scrollTop = chatHistory.scrollHeight;
    };

    // --- UI LOGIC ---
    const showView = (viewToShow) => {
        [welcomeView, conversationView, taskView].forEach(view => {
            view.style.display = view === viewToShow ? (view === conversationView ? 'flex' : 'block') : 'none';
        });
    };

    const selectConversation = async (id) => {
        currentConversationId = id;
        const conv = await api.getConversation(id);
        convTitle.textContent = conv.name;
        renderChatHistory(conv.history);
        showView(conversationView);
        
        document.querySelectorAll('#conversations-list li').forEach(li => {
            li.classList.toggle('active', li.dataset.id === id);
        });
        document.querySelectorAll('#tasks-list li').forEach(li => li.classList.remove('active'));
    };

    const selectTask = async (taskName) => {
        currentConversationId = null;
        const task = await api.getTaskDetails(taskName);
        taskTitle.textContent = `Task: ${task.name}`;
        taskForm.elements.name.value = task.name;
        taskForm.elements.description.value = task.description;
        taskForm.elements.schedule.value = task.schedule;
        taskForm.elements.context_path.value = task.context_path;
        taskForm.elements.data_command.value = task.data_command;
        taskForm.elements.prompt.value = task.prompt;

        const logs = await api.getTaskLogs(taskName);
        taskLogs.textContent = logs.join('\n\n---\n\n');
        taskLogsView.style.display = 'block';
        
        showView(taskView);

        document.querySelectorAll('#tasks-list li').forEach(li => {
            li.classList.toggle('active', li.dataset.name === taskName);
        });
        document.querySelectorAll('#conversations-list li').forEach(li => li.classList.remove('active'));
    };

    const handleNewConversation = async () => {
        const newConv = await api.createConversation();
        await renderConversations();
        selectConversation(newConv.id);
    };

    const handleSendPrompt = async () => {
        const prompt = promptTextarea.value.trim();
        if (!prompt || !currentConversationId) return;

        promptTextarea.value = '';
        promptTextarea.disabled = true;
        sendPromptBtn.disabled = true;

        // Add user message to UI immediately
        const userMessageDiv = document.createElement('div');
        userMessageDiv.className = 'message user';
        userMessageDiv.textContent = prompt;
        chatHistory.appendChild(userMessageDiv);

        // This will hold the thinking box and the final response
        const agentResponseContainer = document.createElement('div');
        chatHistory.appendChild(agentResponseContainer);

        const geminiMessageDiv = document.createElement('div');
        geminiMessageDiv.className = 'message gemini';
        
        const loadingIndicator = document.createElement('div');
        loadingIndicator.className = 'loading-indicator';
        loadingIndicator.textContent = 'Thinking...';
        geminiMessageDiv.appendChild(loadingIndicator);
        agentResponseContainer.appendChild(geminiMessageDiv); // Add gemini message div from the start

        chatHistory.scrollTop = chatHistory.scrollHeight;

        const protocol = window.location.protocol === 'https:' ? 'wss:' : 'ws:';
        const socket = new WebSocket(`${protocol}//${window.location.host}/api/v1/conversations/${currentConversationId}/prompt/stream`);

        let thinkingBox = null;
        let hasReceivedText = false;

        socket.onopen = () => {
            socket.send(prompt);
        };

        socket.onmessage = (event) => {
            if (loadingIndicator.parentNode) {
                loadingIndicator.remove();
            }

            const data = JSON.parse(event.data);
            console.log("Received WS data:", data); // For debugging

            // Helper function to process message parts and display text
            const processMessageParts = (parts) => {
                if (!hasReceivedText) {
                    hasReceivedText = true;
                    if (thinkingBox) {
                        thinkingBox.remove();
                        thinkingBox = null;
                    }
                }
                parts.forEach(part => {
                    if (part.kind === 'text') {
                        geminiMessageDiv.textContent += part.text;
                    }
                });
            };

            switch (data.kind) {
                case 'message':
                    processMessageParts(data.parts);
                    break;

                case 'task':
                    if (!thinkingBox) {
                        thinkingBox = document.createElement('div');
                        thinkingBox.className = 'thinking-box';
                        agentResponseContainer.insertBefore(thinkingBox, geminiMessageDiv);
                    }
                    thinkingBox.innerHTML = `<h5>Task Created</h5><p>Status: ${data.status.state}</p>`;
                    break;

                case 'task_status_update':
                    if (!thinkingBox) {
                        thinkingBox = document.createElement('div');
                        thinkingBox.className = 'thinking-box';
                        agentResponseContainer.insertBefore(thinkingBox, geminiMessageDiv);
                    }
                    thinkingBox.innerHTML = `<h5>Task Status: ${data.status.state}</h5>`;

                    if (data.status.message && data.status.message.parts) {
                        processMessageParts(data.status.message.parts);
                    }
                    break;
                
                case 'task_artifact_update':
                    if (!thinkingBox) {
                        thinkingBox = document.createElement('div');
                        thinkingBox.className = 'thinking-box';
                        agentResponseContainer.insertBefore(thinkingBox, geminiMessageDiv);
                    }
                    thinkingBox.innerHTML = `<h5>Processing Artifact...</h5>`;
                    break;

                default:
                    console.log("Received unhandled event kind:", data.kind);
            }
        };

        socket.onclose = () => {
            if (thinkingBox) {
                thinkingBox.remove();
            }
            promptTextarea.disabled = false;
            sendPromptBtn.disabled = false;
            promptTextarea.focus();
        };

        socket.onerror = (error) => {
            console.error("WebSocket error:", error);
            if (loadingIndicator.parentNode) {
                loadingIndicator.remove();
            }
            geminiMessageDiv.textContent = "Error: Could not get response from server.";
        };
    };

    const handleDeleteConversation = async () => {
        if (!currentConversationId) return;
        if (confirm(`Are you sure you want to delete conversation ${currentConversationId}?`)) {
            await api.deleteConversation(currentConversationId);
            currentConversationId = null;
            await renderConversations();
            showView(welcomeView);
        }
    };

    // --- INITIALIZATION ---
    const init = () => {
        newConvBtn.addEventListener('click', handleNewConversation);
        sendPromptBtn.addEventListener('click', handleSendPrompt);
        deleteConvBtn.addEventListener('click', handleDeleteConversation);
        promptTextarea.addEventListener('keydown', (e) => {
            if (e.key === 'Enter' && !e.shiftKey) {
                e.preventDefault();
                handleSendPrompt();
            }
        });

        taskForm.addEventListener('submit', async (e) => {
            e.preventDefault();
            const taskName = taskForm.elements.name.value;
            const task = {
                name: taskName,
                description: taskForm.elements.description.value,
                schedule: taskForm.elements.schedule.value,
                context_path: taskForm.elements.context_path.value,
                data_command: taskForm.elements.data_command.value,
                prompt: taskForm.elements.prompt.value,
            };
            await api.updateTask(taskName, task);
            alert('Task saved!');
            await renderTasks();
        });

        deleteTaskBtn.addEventListener('click', async () => {
            const taskName = taskForm.elements.name.value;
            if (confirm(`Are you sure you want to delete task ${taskName}?`)) {
                await api.deleteTask(taskName);
                await renderTasks();
                showView(welcomeView);
            }
        });

        renderConversations();
        renderTasks();
        showView(welcomeView);
    };

    const fetchAndDisplayModel = async () => {
        const model = await api.getModel();
        modelInfo.textContent = `Model: ${model.model}`;
    };

    const fetchAndDisplayStats = async () => {
        const stats = await api.getStats();
        statsInfo.textContent = `Calls: ${stats.total_calls} | Avg Latency: ${stats.avg_latency_ms}ms | Chars In: ${stats.total_chars_in} | Chars Out: ${stats.total_chars_out}`;
    };

    init();
    fetchAndDisplayModel();
    fetchAndDisplayStats();
    setInterval(fetchAndDisplayStats, 5000);
});
