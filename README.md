# Gemini Srv

Gemini Srv is a web server and API that provides a user-friendly interface for interacting with the `gemini-cli` tool. It allows for persistent, context-aware conversations and includes a powerful scheduler for running autonomous, scheduled tasks.

![gemini-srv-gif2](https://github.com/user-attachments/assets/124a907f-fdb2-44e7-b984-4549930d546b)

## Use Cases

-   **AI Assistant on a VM at your disposal:** Deploy Gemini Srv on a virtual machine to have a dedicated AI assistant accessible from anywhere.
-   **Your custom Gemini-CLI agent integrated to Google Chat, Slack or Teams:** Integrate the Gemini Srv API with your favorite communication platforms to bring your custom Gemini-CLI agent directly into your team's workflow.
-   **Automated Task Execution:** Leverage the scheduler to run autonomous tasks that gather data, interact with the Gemini model, and perform actions based on predefined conditions.
-   **Scheduled Reporting and Alerts:** Configure scheduled tasks to generate reports or send alerts based on specific criteria, powered by Gemini's analytical capabilities.
-   **Interactive Web UI for Gemini-CLI:** Provides a user-friendly web interface to interact with your Gemini-CLI agent, manage conversations, and monitor scheduled tasks.



## Features

-   **Web UI:** A clean, modern web interface for managing conversations and tasks.
-   **REST API:** A simple, stateless API for easy integration with other services like Slack, Teams, or custom scripts.
-   **Persistent Conversations:** Conversation histories are saved to disk, so you can pick up where you left off.
-   **Scheduled Tasks:** Define autonomous tasks in `.toml` files that can gather data and conditionally call the Gemini model.
-   **Context-Aware:** Leverages `gemini-cli`'s ability to read context from `GEMINI.md` files.

## Getting Started

### Prerequisites

-   Go 1.18+
-   A working `gemini-cli` installation.

### Installation & Running

1.  **Clone the repository:**
    ```bash
    git clone <repository_url>
    cd gemini-srv
    ```

2.  **Create a `.env` file:**
    Copy the example file and customize it with your details.
    ```bash
    cp .env.example .env
    # Edit .env with your preferred user/pass and the correct path to gemini-cli
    ```

3.  **Build the server:**
    ```bash
    go build .
    ```

4.  **Run the server directly (for testing):**
    ```bash
    ./gemini-srv
    ```

5.  Open your browser and navigate to `http://localhost:7123`.

## Running as a Service (systemd)

To run `gemini-srv` as a persistent background service that starts on boot, you can create a `systemd` unit file.

1.  **Create the service file:**
    Create a new file at `/etc/systemd/system/gemini-srv.service`:
    ```bash
    sudo nano /etc/systemd/system/gemini-srv.service
    ```

2.  **Add the following content:**
    Be sure to replace `/path/to/your/gemini-srv` with the actual absolute path to the project directory. The `WorkingDirectory` is crucial as it tells the service where to find the `.env` file, the `static/` directory, and the `data/` directory.

    ```ini
    [Unit]
    Description=Gemini Srv - A web UI and API for gemini-cli
    After=network.target

    [Service]
    Type=simple
    User=your_username # Replace with the user you want to run the service as
    Group=your_groupname # Replace with the group for that user

    # Set the working directory to the project root
    WorkingDirectory=/path/to/your/gemini-srv

    # Command to start the server
    ExecStart=/path/to/your/gemini-srv/gemini-srv

    Restart=on-failure
    RestartSec=5s

    [Install]
    WantedBy=multi-user.target
    ```

3.  **Enable and start the service:**
    ```bash
    # Reload the systemd daemon to recognize the new file
    sudo systemctl daemon-reload

    # Start the service now
    sudo systemctl start gemini-srv

    # Enable the service to start automatically on boot
    sudo systemctl enable gemini-srv
    ```

4.  **Check the status and logs:**
    ```bash
    # Check if the service is running
    sudo systemctl status gemini-srv

    # View the live logs
    sudo journalctl -u gemini-srv -f
    ```

## API Usage

The server exposes a simple REST API for integrations.

-   `POST /api/v1/conversations`: Create a new conversation.
-   `GET /api/v1/conversations`: List all conversation IDs.
-   `GET /api/v1/conversations/{id}`: Get the history of a conversation.
-   `POST /api/v1/conversations/{id}/prompt`: Send a prompt to a conversation.
-   `DELETE /api/v1/conversations/{id}`: Delete a conversation.

All API endpoints are protected by Basic Authentication using the credentials set in your `.env` file.
