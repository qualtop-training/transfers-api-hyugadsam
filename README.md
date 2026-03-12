# Transfers API — Project Template

This repository provides the **initial structure** for the *Money Transfers API* used during the Go Microservices training.

The goal of this template is to start from a **clean, reproducible baseline** so every participant works under the same conditions.

---

## Purpose

This template includes only the minimum required setup:

* Go module initialized
* Standard project structure
* Basic application entrypoint
* Docker local environment
* Ready to evolve session by session

Business logic will be implemented progressively during the training.

---

## Initial Project Structure

```
cmd/            → application entrypoints
internal/       → application code
pkg/            → optional shared packages
```

> Structure will evolve as the course progresses.

---

## Requirements

* Go (latest stable version recommended)
* Docker
* Git

Verify installation:

```
go version
docker version
```

---

## Getting Started

### 1. Create your repository from this template

Click **Use this template** and create your personal repository inside the organization.

---

### 2. Clone your repository

```
git clone <your-repo-url>
cd transfers-api
```

---

### 3. Run the application

```
go run ./cmd/api
```

*(Path may change during the training.)*

---

### 4. Run with Docker

```
docker compose up --build
```

---

### 5. Test MongoDB in Docker

```bash
docker exec -it {containerID} mongo -u root -p root --authenticationDatabase admin
show dbs
use transfers-db
db.transfers.find().pretty()
db.transfers.count()
```

### 5. Test MySQL in Docker

```bash
docker exec -it transfers-mysql mysql -u root -p
show databases;
use transfers-db;
CREATE TABLE transfers (
	id BIGINT AUTO_INCREMENT PRIMARY KEY,
	sender_id VARCHAR(255) NOT NULL,
	receiver_id VARCHAR(255) NOT NULL,
	currency VARCHAR(10) NOT NULL,
	amount DOUBLE NOT NULL,
	state VARCHAR(50) NOT NULL,
	created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
	updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP
);
SELECT * FROM transfers;
```

### Test Memcached in Docker


## Training Workflow

* Each session introduces new concepts
* Code will evolve incrementally
* You are expected to modify and extend the project between sessions

This repository is your **working environment**, not a finished solution.

---

## Guidelines

* Keep commits small and meaningful
* Prefer clarity over clever solutions
* Ask questions early
* Experiment freely

---

## License

Educational use only.