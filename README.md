# Kanso Sync Engine

![Go Version](https://img.shields.io/badge/Go-1.22+-00ADD8?style=flat&logo=go)
![Docker](https://img.shields.io/badge/Docker-Enabled-2496ED?style=flat&logo=docker)
![Architecture](https://img.shields.io/badge/Architecture-Hexagonal-orange)
![License](https://img.shields.io/badge/License-MIT-green)

**Kanso Sync Engine** is the high-performance backend infrastructure for *Kanso*, an Offline-First Habit Tracker.

Designed as a distributed system rather than a simple CRUD API, this service acts as the authoritative source of truth for the Kanso ecosystem. It handles complex synchronization logic, conflict resolution strategies, and timezone-aware aggregations, ensuring data consistency across multiple client devices even in unreliable network conditions.

---

## Architectural Design & Engineering Decisions

The system is built upon **Hexagonal Architecture (Ports & Adapters)**. This design choice isolates the core domain logic from external concerns (Database, HTTP Transport), ensuring that business rules are pure, testable, and dependency-free.

### 1. Offline-First Synchronization (Delta-Sync)
To support a seamless offline experience, the engine implements a **Cursor-Based Delta Sync** mechanism.
* **The Challenge:** Efficiently propagating changes to client devices without sending the entire dataset.
* **The Solution:** Clients request synchronization using a timestamp cursor. The server queries the repository for records modified *after* that cursor, returning only the deltas.
* **Tombstones:** To handle deletions in a distributed system, records are not physically removed but marked via **Soft Deletes** (`deleted_at`). This allows "deletion events" to propagate to other devices during sync, ensuring strict consistency.

### 2. Concurrency Control (Optimistic Locking)
In a multi-device environment, data races are a critical risk (e.g., modifying a habit on a tablet while the phone is syncing).
* **The Challenge:** Preventing the "Lost Update Problem" without locking the database for long periods.
* **The Solution:** The system implements **Optimistic Locking** via versioning. Every write operation requires the client to provide the current version of the entity. The database atomically verifies `UPDATE ... WHERE version = client_version`. If a conflict is detected, the transaction is rejected, forcing the client to pull the latest data before retrying.

### 3. Global Timezone & DST Awareness
Habit trackers often suffer from the "Midnight Bug"—where a habit completed at 1 AM in Rome counts for the previous day in London.
* **The Challenge:** Accurately calculating streaks and weekly stats for users in different timezones.
* **The Solution:**
    * **Storage:** strictly **UTC** to maintain a monotonic timeline.
    * **Aggregation:** The `StatsService` accepts the user's IANA Timezone (e.g., `Europe/Rome`). Data is projected into the user's local time *before* aggregation logic runs. This handles Daylight Saving Time (DST) transitions automatically and ensures "Midnight" is always relative to the user's physical location.

### 4. Resilience & Background Processing
* **Graceful Shutdown:** The application handles OS signals (`SIGTERM`, `SIGINT`) to ensure zero-downtime deployments. It utilizes `sync.WaitGroup` to drain active connections and allows background workers to finish their current jobs before the process exits.
* **Asynchronous Workers:** Heavy calculations (like Streak updates) are offloaded to background workers via buffered channels, keeping the HTTP response times low.
* **Rate Limiting:** To protect the system from abuse or accidental DDo, a Redis-based Rate Limiter middleware throttles excessive requests based on IP.

---

## Tech Stack

* **Language:** Go (Golang) 1.22
* **Framework:** Gin Web Framework
* **Database:** PostgreSQL 16 (w/ `pgx` driver & `sqlx`)
* **Caching/Limiting:** Redis 7
* **Authentication:** JWT (JSON Web Tokens) with RS256/HS256
* **Containerization:** Docker & Docker Compose
* **CI/CD:** GitHub Actions (Automated Testing & Linting)

---

## Development Philosophy & AI-Augmentation

This project serves as a case study in **modern, AI-augmented software engineering**.

While I am a Computer Science student, my goal was to bridge the gap between academic theory and production-grade engineering standards. To achieve this, I integrated Large Language Models (LLMs) into my workflow as a "Senior Technical Partner."

* **Role of AI:** I leveraged AI to accelerate the implementation of boilerplate adapters, generate comprehensive integration test suites (covering edge cases like DST transitions), and validate my architectural constraints.
* **My Role:** I was responsible for the **System Design**, **Constraint Definitions**, and **Core Logic**. Every line of code—whether hand-written or generated—has been rigorously reviewed, tested, and reverse-engineered to ensure a deep understanding of the underlying mechanics, from Go memory models to PostgreSQL isolation levels.

This repository represents the engineering standard I aspire to: robust, scalable, and built with a deep understanding of the problem domain.

---

## Getting Started

### Prerequisites
* Docker & Docker Compose
* Go 1.22+ (optional, for local dev)

### Running with Docker (Recommended)

1.  **Clone the repository**
    ```bash
    git clone [https://github.com/yourusername/kanso-backend.git](https://github.com/yourusername/kanso-backend.git)
    cd kanso-backend
    ```

2.  **Start the stack**
    This command spins up the Go API, PostgreSQL, and Redis containers.
    ```bash
    docker-compose up --build
    ```

3.  **Access the API**
    The server will start on port `8080`.
    * Health Check: `http://localhost:8080/health`
    * Swagger Documentation: `http://localhost:8080/swagger/index.html`

### Running Tests
To run the full suite of unit and integration tests:

```bash
go test -v -race ./...
```

## API Documentation

The API is fully documented using Swagger. Once the server is running, navigate to http://localhost:8080/swagger/index.html