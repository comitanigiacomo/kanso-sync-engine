# Kanso Sync Engine

![Go Version](https://img.shields.io/badge/Go-1.22+-00ADD8?style=flat&logo=go)
![Docker](https://img.shields.io/badge/Docker-Enabled-2496ED?style=flat&logo=docker)
![License](https://img.shields.io/badge/License-MIT-green)
![Tests](https://img.shields.io/badge/Tests-Comprehensive-brightgreen)

**Kanso Sync Engine** is the backend for *Kanso*, an Offline-First Habit Tracker built with Go.

It implements a comprehensive synchronization system designed to handle data synchronization across multiple devices, conflict resolution, and timezone management in unreliable network conditions. The entire codebase is thoroughly tested with comprehensive unit and integration tests.

---

## Architecture Overview

The system is built using **Hexagonal Architecture** to maintain separation of concerns between business logic and technical infrastructure (database, API layer), improving code maintainability and testability.

### Core Features


- **Offline-First Synchronization (Delta-Sync)**: The synchronization mechanism uses delta-sync to optimize data transfer. Clients submit a timestamp cursor to request only records modified since the last synchronization. The server returns only new or updated records. Deletions are implemented as soft deletes to maintain consistency.

- **Conflict Resolution (Optimistic Locking)**: Concurrent modifications are resolved using versioning. Each record maintains a version number; update requests include the current version. Version mismatches trigger rejection (409 Conflict), requiring clients to pull the latest state before retrying.

- **Timezone Awareness**: All data is stored in UTC. Statistical aggregations accept the user's IANA Timezone (e.g., Europe/Rome) via headers to correctly calculate daily progress based on local time, solving the "Midnight Bug".

- **Reliability & Performance**

    - *Graceful Shutdown*: Workers drain active jobs using sync.WaitGroup before the application exits.

    - *Async Processing*: Heavy computations (like streak calculations) are offloaded to background workers.

    - *Rate Limiting*: Redis-based token bucket algorithm to prevent abuse.

---

## Technology Stack

| Component | Technology |
|-----------|-----------|
| **Language** | Go 1.22+ |
| **Framework** | Gin |
| **Database** | PostgreSQL 16 |
| **Cache/Rate Limiting** | Redis 7 |
| **Authentication** | JWT (RS256/HS256) |
| **Containerization** | Docker & Docker Compose |
| **CI/CD** | GitHub Actions |

---

## Project Structure

The codebase reflects the Hexagonal separation:

```text
kanso-sync-engine/
├── .github/            
├── cmd/api/            
├── db/                 
├── docs/               
├── internal/
│   ├── core/          
│   │   ├── domain/   
│   │   ├── services/   
│   │   └── workers/    
│   └── adapters/       
│       ├── cache/      
│       ├── handler/    
│       ├── repository/ 
│       └── storage/    
├── pkg/
├── docker-compose.yml
└── ...
```
---

## Getting Started

### Prerequisites
Docker & Docker Compose; Go 1.22+ (optional, for local development).

### Docker Deployment

1. **Clone the repository**
    ```bash
    git clone https://github.com/yourusername/kanso-backend.git
    cd kanso-backend
    ```

2. **Initialize services**
    ```bash
    docker-compose up --build
    ```

3. **Access endpoints:** Health check at `http://localhost:8080/health` and API documentation at `http://localhost:8080/swagger/index.html`

### Running Tests
```bash
go test -v -race ./...
```

## API Documentation

Complete API specification available at `http://localhost:8080/swagger/index.html`

## Acknowledgments

*AI tools were used to assist with testing and code generation to speed up development.*