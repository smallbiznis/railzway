# Deployment Guide

This guide explains how to deploy Railzway services using Docker and Docker Compose.

## 🐳 Docker Images

We publish the following images to GitHub Container Registry (GHCR):

| Service | Image | Description |
| :--- | :--- | :--- |
| **Monolith** | `ghcr.io/smallbiznis/railzway/railzway` | All-in-one binary (Admin UI + API + Scheduler). Best for simple deployments. |
| **Admin** | `ghcr.io/smallbiznis/railzway/railzway-admin` | Admin Dashboard (UI) + API. No background jobs. |
| **Scheduler** | `ghcr.io/smallbiznis/railzway/railzway-scheduler` | Background workers (Rating, Invoicing). No UI. |
| **Invoice** | `ghcr.io/smallbiznis/railzway/railzway-invoice` | Public Invoice Rendering (customer-facing). |
| **API** | `ghcr.io/smallbiznis/railzway/railzway-api` | Headless API service. |

## 🚀 Running with Docker Compose

Create a `docker-compose.yml` file with the following content:

```yaml
services:
  # Database
  postgres:
    image: postgres:15-alpine
    environment:
      POSTGRES_USER: railzway
      POSTGRES_PASSWORD: password
      POSTGRES_DB: railzway
    volumes:
      - postgres_data:/var/lib/postgresql/data
    ports:
      - "5432:5432"

  # Infrastructure
  redis:
    image: redis:7-alpine
    ports:
      - "6379:6379"

  # -----------------------------------------------------
  # Option A: Monolith (All-in-One)
  # -----------------------------------------------------
  # railzway:
  #   image: ghcr.io/smallbiznis/railzway/railzway:latest
  #   ports:
  #     - "8080:8080"
  #   environment:
  #     - DB_HOST=postgres
  #     - DB_USER=railzway
  #     - DB_PASSWORD=password
  #     - REDIS_HOST=redis
  #   depends_on:
  #     - postgres
  #     - redis

  # -----------------------------------------------------
  # Option B: Microservices (Recommended for Production)
  # -----------------------------------------------------
  
  # 1. Admin Dashboard (UI + API)
  admin:
    image: ghcr.io/smallbiznis/railzway/railzway-admin:latest
    ports:
      - "8080:8080"
    environment:
      - PORT=8080
      - DB_HOST=postgres
      - REDIS_HOST=redis
    depends_on:
      - postgres
      - redis

  # 2. Scheduler (Background Jobs)
  scheduler:
    image: ghcr.io/smallbiznis/railzway/railzway-scheduler:latest
    environment:
      - ENABLED_JOBS=billing,usage,invoice # Run all jobs
      - DB_HOST=postgres
      - REDIS_HOST=redis
    depends_on:
      - postgres
      - redis

  # 3. Public Invoice Service
  invoice:
    image: ghcr.io/smallbiznis/railzway/railzway-invoice:latest
    ports:
      - "3000:8080" # Exposed on port 3000
    environment:
      - PORT=8080
      - API_URL=http://admin:8080 # Points to Admin API for data
    depends_on:
      - admin

volumes:
  postgres_data:
```

### Start the Stack
```bash
docker-compose up -d
```

## 📦 Running Individual Containers

If you prefer `docker run`:

### 1. Run Admin Service
```bash
docker run -d \
  --name railzway-admin \
  -p 8080:8080 \
  -e DB_HOST=host.docker.internal \
  -e REDIS_HOST=host.docker.internal \
  ghcr.io/smallbiznis/railzway/railzway-admin:latest
```

### 2. Run Scheduler
```bash
docker run -d \
  --name railzway-scheduler \
  -e DB_HOST=host.docker.internal \
  -e REDIS_HOST=host.docker.internal \
  ghcr.io/smallbiznis/railzway/railzway-scheduler:latest
```

> Note: `host.docker.internal` allows the container to access services running on your host machine (like local Postgres).

## Environment Variables

| Variable | Description | Default |
| :--- | :--- | :--- |
| `PORT` | HTTP Port to listen on | `8080` |
| `DB_HOST` | Postgres Host | `localhost` |
| `DB_USER` | Postgres User | `postgres` |
| `DB_NAME` | Postgres DB Name | `postgres` |
| `REDIS_HOST` | Redis Host | `localhost` |
| `ENABLED_JOBS` | Comma-separated list of jobs (Scheduler only) | All jobs |
