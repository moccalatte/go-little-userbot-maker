# Project Architecture

This document provides a high-level overview of the Go Little Userbot Maker project architecture. The goal of this structure is to create a clear separation of concerns by organizing the project into self-contained services, making the codebase easier to understand, maintain, and test.

We follow the principles of **Clean Architecture** within each service.

## Top-Level Directory Structure

-   **/services**: Contains the individual, self-contained services of the project. Each service has its own `cmd` (entrypoint) and `internal` (core logic) directories.
    -   **/services/bot-wizard**: The Telegram bot interface for users.
    -   **/services/userbot-orchestrator**: The backend service that manages userbot sessions.

-   **/pkg**: Contains shared libraries and utilities that can be used by multiple services.
    -   **/pkg/logger**: A simple, file-based logger.
    -   **/pkg/config**: Shared configuration loading logic.
    -   **/pkg/storage**: Shared database connection logic.

-   **/docs**: Contains all project documentation, including this file, the README, and product requirements.

-   **/scripts**: Contains helper scripts for development tasks.

-   **/logs**: The directory where all log files are written. This directory is created at runtime and is not checked into version control.

## Application Layers (Clean Architecture within each service)

Both the `bot-wizard` and `userbot-orchestrator` services follow a three-layer architecture:

1.  **Delivery Layer (`internal/delivery/`)**
    -   **Responsibility**: Handles incoming requests from the outside world. This is the outermost layer.
    -   **Examples**:
        -   In the **bot-wizard**, this layer is `services/bot-wizard/internal/delivery/telegram/`, which contains the `handler.go` for processing incoming Telegram bot updates.
        -   In the **userbot-orchestrator**, this is `services/userbot-orchestrator/internal/delivery/http/`, which contains the `handler.go` for managing the public HTTP API endpoints.
    -   **Rule**: This layer knows about the `usecase` layer but not the `repository` layer. It translates external requests into calls to the usecase.

2.  **Usecase Layer (`internal/usecase/`)**
    -   **Responsibility**: Contains the core business logic of the application. It orchestrates the flow of data and implements the application-specific rules.
    -   **Examples**:
        -   `services/bot-wizard/internal/usecase/wizard_usecase.go` handles the conversational flow for creating a userbot.
        -   `services/userbot-orchestrator/internal/usecase/session_usecase.go` manages the lifecycle of userbot sessions (creating, deleting, starting workers).
    -   **Rule**: This is the central layer. It does not depend on any other layer. It defines interfaces that the `repository` and `delivery` layers must implement.

3.  **Repository Layer (`internal/repository/`)**
    -   **Responsibility**: Manages all data persistence and communication with external services. It implements the interfaces defined by the `usecase` layer.
    -   **Examples**:
        -   `services/bot-wizard/internal/repository/state.go` provides an in-memory store for the bot's conversational state.
        -   `services/userbot-orchestrator/internal/repository/session_repo.go` handles all SQL queries for reading and writing session data to the PostgreSQL database.
    -   **Rule**: This layer depends on the `usecase` layer (by implementing its interfaces) but knows nothing about the `delivery` layer.

## Logging System

The project uses a simple, custom file-based logging system located in `pkg/logger`. This system is designed to be lightweight and language-agnostic in its output format.

-   **Log File Structure**: Logs are stored in `./logs/{service_name}/{telegram_id}/{date}.log`.
    -   `service_name`: The name of the service that generated the log (e.g., `bot-wizard`, `userbot-orchestrator`).
    -   `telegram_id`: The Telegram user ID associated with the log entry. For service-wide logs, this is set to `main`. For user-specific logs (like a userbot worker), this is the user's ID.
    -   `date`: The date in `YYYY-MM-DD` format. Logs are rotated daily.
-   **Log Format**: `[timestamp] [level] [service] [telegram_id] message`
    -   Example: `[2025-10-05T14:35:08Z] [INFO] [userbot-orchestrator] [main] starting userbot-orchestrator service`
-   **Debugging**: This consistent format allows developers and AI assistants to easily parse log files, trace requests across services, and debug issues by examining the log files for a specific user.

## How It Fits Together: An Example Flow (Creating a Userbot)

1.  A user sends a message to the Telegram bot.
2.  The `bot-wizard` service's main loop (`services/bot-wizard/cmd/main.go`) receives the update.
3.  The `services/bot-wizard/internal/delivery/telegram/handler.go` receives the update and calls the `HandleUpdate` method on the `services/bot-wizard/internal/usecase/wizard_usecase.go`.
4.  The `wizard_usecase` processes the logic and uses its `StateRepository` interface to save the user's progress.
5.  The `services/bot-wizard/internal/repository/state.go` (which implements the `StateRepository` interface) saves the data in memory.
6.  When the flow is complete, the `wizard_usecase` calls the `OrchestratorRepository` interface to create the session.
7.  The `services/bot-wizard/internal/repository/orchestrator_client.go` (which implements the interface) makes an HTTP call to the `userbot-orchestrator` service's `/sessions` endpoint.
8.  The request is received by `services/userbot-orchestrator/internal/delivery/http/handler.go`.
9.  The handler calls the `CreateSession` method on the `services/userbot-orchestrator/internal/usecase/session_usecase.go`.
10. The `session_usecase` encrypts the session data and calls the `SessionRepository` interface to save it.
11. The `services/userbot-orchestrator/internal/repository/session_repo.go` executes the SQL query to insert the new session into the database.

This layered and service-oriented approach makes the system modular and easier to understand, as each component has a single, well-defined responsibility.