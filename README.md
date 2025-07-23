# LoadBalancer Project

This repository contains a scalable load balancer system implemented in Go, paired with a .NET 9.0 API backend and accompanying load-testing scripts.

## Table of Contents

* [Project Overview](#project-overview)
* [Prerequisites](#prerequisites)
* [Project Structure](#project-structure)
* [Installation](#installation)

  * [API Service](#api-service)
  * [Load Balancer (Go App)](#load-balancer-go-app)
* [Running the Services](#running-the-services)

  * [Using Docker Compose](#using-docker-compose)
  * [Manual Startup](#manual-startup)
* [Load Testing](#load-testing)
* [Logs & Metrics](#logs--metrics)
* [Database Migrations](#database-migrations)
* [License](#license)

---

## Project Overview

* **API Service**: A .NET 9.0 C# Web API that exposes endpoints for user operations, backed by a relational database and EF Core migrations.
* **Load Balancer**: A Go-based microservice that distributes incoming requests across healthy API replicas, with auto-scaling logic, Active Monitoring and health checks.
* **Load Testing**: JavaScript-based test scripts and HTML/JSON reports to simulate traffic and validate performance.

## Prerequisites

* [.NET 9.0 SDK](https://dotnet.microsoft.com/download) (for API)
* [Go 1.21+](https://go.dev/dl/) (for load balancer)
* [Docker & Docker Compose](https://docs.docker.com/get-started/) (optional, for containerized setup)
* [k6](https://k6.io/) or Node.js (if using JavaScript benchmarks)

## Project Structure

```
loadBalancer/
├── API/                   # .NET 9.0 Web API project
│   ├── Dockerfile         # Builds API container
│   ├── docker-compose.yaml# Defines API + DB services
│   ├── Program.cs
│   ├── API.csproj
│   └── database/          # Connection & EF Core migrations
│       ├── connection.cs
│       └── migration/     # *.cs scripts for schema
├── App/                   # Go load balancer microservice
│   ├── main.go            # Entry point
│   ├── config/            # Global config definitions
│   ├── functions/         # Scaling, health checks, routing
│   ├── structers/         # Data models & heaps
│   └── graphs/Logs        # Latency logs & charting tools
├── Test/                  # Load-testing scripts & results
│   ├── loadtest.js        # k6 JS tests
│   ├── report.html/json   # Generated performance reports
│   └── benchmarks/        # HTML summaries (V1–V7)
├── go.mod
├── go.sum
├── loadBalancer.sln       # Visual Studio solution
├── .gitignore
└── LICENSE
```

## Installation

### API Service

1. Navigate to the API directory:

   ```bash
   cd loadBalancer/API
   ```
2. Restore dependencies and build:

   ```bash
   dotnet restore
   dotnet build -c Release
   ```

### Load Balancer (Go App)

1. Navigate to the App directory:

   ```bash
   cd loadBalancer/App
   ```
2. Download dependencies and build:

   ```bash
   go mod tidy
   go build -o loadbalancer
   ```

## Running the Services

By default, when the load balancer starts it will automatically create any missing API service instances and initialize the database.

```bash
cd loadBalancer/APP
go run main.go
```

## Load Testing

Use the provided JavaScript load test (`loadBalancer/Test/loadtest.js`) with k6 or Node.js:

```bash
cd loadBalancer/Test
# or if built: ./loadbalancer
k6 run loadtest.js
```

Review the generated reports in `report.html` or JSON outputs.

## Logs & Metrics

* Latency logs are written to `loadBalancer/App/Logs/latency.log`.
* Charts can be generated via `App/graphs/chart_shower.go` (requires Go plotting libraries).

## Database Migrations

Migration scripts are located in `API/database/migration/`. To apply:

```bash
cd loadBalancer/API
dotnet ef database update
```

## License

This project is licensed under the Apache License 2.0. See the [LICENSE](LICENSE) file for details.

## Contact

Maintained by **Belqadi Ayman**. Feel free to open issues or reach out via GitHub.

