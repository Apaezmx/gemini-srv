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
    const taskLogs = document.getElementById('task-logs').querySelector('pre');

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
        postPrompt: (id, prompt) => fetch(`/api/v1/conversations/${id}/prompt`, {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify({ prompt }),
        }).then(res => res.json()),
        getTasks: () => fetch('/api/v1/tasks').then(res => res.json()),
        getTaskLogs: (taskName) => fetch(`/api/v1/tasks/${taskName}/logs`).then(res => res.json()),
    };

    // --- RENDER FUNCTIONS ---
    const renderConversations = async () => {
        const ids = await api.getConversations();
        conversationsList.innerHTML = '';
        ids.forEach(id => {
            const li = document.createElement('li');
            li.textContent = id;
            li.dataset.id = id;
            li.addEventListener('click', () => selectConversation(id));
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
            const [type, ...contentParts] = line.split(': ');
            const content = contentParts.join(': ');
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
        convTitle.textContent = `Conversation: ${id}`;
        renderChatHistory(conv.history);
        showView(conversationView);
        
        document.querySelectorAll('#conversations-list li').forEach(li => {
            li.classList.toggle('active', li.dataset.id === id);
        });
        document.querySelectorAll('#tasks-list li').forEach(li => li.classList.remove('active'));
    };

    const selectTask = async (taskName) => {
        currentConversationId = null;
        const logs = await api.getTaskLogs(taskName);
        taskTitle.textContent = `Logs for: ${taskName}`;
        taskLogs.textContent = logs.join('\n\n---\n\n');
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

        // Add user message and loading indicator to UI immediately
        const userMessageDiv = document.createElement('div');
        userMessageDiv.className = 'message user';
        userMessageDiv.textContent = prompt;
        chatHistory.appendChild(userMessageDiv);

        const loadingDiv = document.createElement('div');
        loadingDiv.className = 'message gemini loading';
        chatHistory.appendChild(loadingDiv);
        chatHistory.scrollTop = chatHistory.scrollHeight;

        let statementIndex = 0;
        loadingDiv.textContent = funStatements[statementIndex];
        const intervalId = setInterval(() => {
            statementIndex = (statementIndex + 1) % funStatements.length;
            loadingDiv.textContent = funStatements[statementIndex];
        }, 1500);

        try {
            await api.postPrompt(currentConversationId, prompt);
        } catch (error) {
            console.error("Failed to send prompt:", error);
            loadingDiv.textContent = "Error: Could not get response from server.";
            loadingDiv.classList.remove('loading');
        } finally {
            clearInterval(intervalId);
            // Refresh the entire chat history from the server to ensure consistency
            const conv = await api.getConversation(currentConversationId);
            renderChatHistory(conv.history);

            promptTextarea.disabled = false;
            sendPromptBtn.disabled = false;
            promptTextarea.focus();
        }
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

        renderConversations();
        renderTasks();
        showView(welcomeView);
    };

    init();
});
