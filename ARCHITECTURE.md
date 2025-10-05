# Project Architecture

This document provides a high-level overview of the Go Little Userbot Maker project architecture. The goal of this structure is to create a clear separation of concerns, making the codebase easier to understand, maintain, and test, especially for new developers.

We follow the principles of **Clean Architecture**, which organizes the code into distinct layers.

## Top-Level Directory Structure

-   **/cmd**: Contains the `main` packages for each executable binary (`botwizard` and `userbot`). These are the entry points of the application. They are responsible for reading configurations, initializing services, and starting them.

-   **/internal**: Holds the core application logic. This code is considered private to this project and cannot be imported by other Go projects. It is further divided into the two main services:
    -   **/internal/wizard**: The Telegram bot interface for users.
    -   **/internal/orchestrator**: The backend service that manages userbot sessions.

-   **/pkg**: Contains shared libraries and utilities that can be used by multiple parts of the application (and could theoretically be used by other projects).
    -   **/pkg/logging**: Shared logging configuration.
    -   **/pkg/storage**: Shared database and Redis connection logic.

-   **/scripts**: Contains helper scripts for tasks like database migrations.

## Application Layers (Clean Architecture)

Both the `wizard` and `orchestrator` modules follow a three-layer architecture:

1.  **Delivery Layer (`delivery/`)**
    -   **Responsibility**: Handles incoming requests from the outside world. This is the outermost layer.
    -   **Examples**:
        -   In the **wizard**, this layer is `delivery/telegram/`, which contains the `handler.go` for processing incoming Telegram bot updates.
        -   In the **orchestrator**, this is `delivery/http/`, which contains the `handler.go` for managing the public HTTP API endpoints.
    -   **Rule**: This layer knows about the `usecase` layer but not the `repository` layer. It translates external requests into calls to the usecase.

2.  **Usecase Layer (`usecase/`)**
    -   **Responsibility**: Contains the core business logic of the application. It orchestrates the flow of data and implements the application-specific rules.
    -   **Examples**:
        -   `wizard/usecase/wizard_usecase.go` handles the conversational flow for creating a userbot.
        -   `orchestrator/usecase/session_usecase.go` manages the lifecycle of userbot sessions (creating, deleting, starting workers).
    -   **Rule**: This is the central layer. It does not depend on any other layer. It defines interfaces that the `repository` and `delivery` layers must implement.

3.  **Repository Layer (`repository/`)**
    -   **Responsibility**: Manages all data persistence and communication with external services. It implements the interfaces defined by the `usecase` layer.
    -   **Examples**:
        -   `wizard/repository/state.go` provides an in-memory store for short-lived conversational state.
        -   `wizard/repository/wizard_persistence.go` persists wizard runs, step snapshots, and the default command presets to PostgreSQL.
        -   `orchestrator/repository/session_repo.go` handles all SQL queries for reading and writing session data to the PostgreSQL database and writes audit trails.
    -   **Rule**: This layer depends on the `usecase` layer (by implementing its interfaces) but knows nothing about the `delivery` layer.

## How It Fits Together: An Example Flow (Creating a Userbot)

1.  A user sends a message to the Telegram bot.
2.  The `wizard` service's main loop receives the update.
3.  The `wizard/delivery/telegram/handler.go` receives the update and calls the `HandleUpdate` method on the `wizard/usecase/wizard_usecase.go`.
4.  The `wizard_usecase` processes the logic (e.g., asks the next question), writes the conversational snapshot to the in-memory `StateRepository`, and mirrors long-running progress to the SQL-backed `WizardPersistence` repository.
5.  When the flow is complete, the `wizard_usecase` calls the `OrchestratorRepository` interface to create or update the session and receives the persisted session identifier in response.
6.  The `wizard/repository/orchestrator_client.go` (which implements the interface) makes an HTTP call to the `orchestrator` service's `/sessions` endpoint.
7.  The request is received by `orchestrator/delivery/http/handler.go`.
8.  The handler calls the `CreateSession` method on the `orchestrator/usecase/session_usecase.go`.
9.  The `session_usecase` encrypts the session data, persists it through the `SessionRepository`, and starts/refreshes the worker.
10. The `orchestrator/repository/session_repo.go` upserts the session, writes an audit log entry, and returns the canonical session identifier.
11. The wizard records the completion status (or failure) in `wizard_runs`, seeds default bot commands via `bot_commands`, and confirms success to the user.

This layered approach makes the system modular and easier to understand, as each component has a single, well-defined responsibility.
