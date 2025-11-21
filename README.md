# hzchat-server - HZ Chat Real-Time Go Backend

## Overview

`hzchat-server` is the official backend service for the HZ Chat real-time application. It is built using the Go language and is specifically designed to handle high-concurrency WebSocket connections and real-time message broadcasting.

Our core goal is to provide users with a secure, private, and instantly available temporary communication environment, and to ensure a transparent and verifiable commitment to user privacy through fully open-sourced code (including both frontend and backend).

> **Frontend Client:** This backend service is designed to work with the [hzchat-web](https://github.com/hezhichaogt/hzchat-web) client.

## License and Trust

| Type | Status |
| :------------- | :-------------------------------------------------------------------------------------------------------------------------------- |
| **License** | [![License](https://img.shields.io/badge/License-AGPL%20v3-blue.svg)](https://www.gnu.org/licenses/agpl-3.0) |
| **Tech Stack** | [![Go](https://img.shields.io/badge/Go-1.25%2B-00ADD8?logo=go)](https://golang.org/) [![WebSocket](https://img.shields.io/badge/Protocol-WebSocket-informational)](https://en.wikipedia.org/wiki/WebSocket) |

**Why AGPL v3.0?**

All code in this project (including the backend core and frontend interface) is open-sourced under the AGPL v3.0 license. This means you can inspect our code at any time to verify that we are not collecting any private data or retaining any backdoors. Our commitment to user privacy is fully transparent and verifiable.

## Core Technology & Architecture

The design of `hzchat-server` is based on Go's fundamental Goroutine and Channel mechanisms, achieving a highly efficient concurrency model.

### Key Features

* **High Concurrency:** Utilizes Go's lightweight concurrency model (goroutines) to effortlessly handle thousands of concurrent WebSocket connections.
* **Real-Time Delivery:** Relies on the WebSocket protocol to ensure low-latency, instantaneous message delivery.
* **State Isolation:** Chat Room instances are completely isolated, with each room running its own independent event loop.
* **Zero Persistence:** No database is used. All chat data remains exclusively in memory. Messages and session states are immediately destroyed when a user leaves or a room is closed due to inactivity timeout, perfectly realizing a privacy-first ephemeral session.

### Backend Data Flow Model

To achieve high concurrency and reliable message passing, `hzchat-server` adopts the following concurrency model:

1.  **Manager:** Responsible for maintaining the map of all active Room instances, handling room creation requests, and running an asynchronous cleanup loop to remove inactive rooms.
2.  **Room:** Each Room runs within a dedicated Goroutine, acting as the message relay hub. It receives client events and messages via Channels (`register`, `unregister`, `broadcast`), ensuring thread safety for all concurrent operations.
3.  **Client:** Each client connection also operates using two separate Goroutines:
    * `ReadPump`: Listens for inbound messages sent by the client.
    * `WritePump`: Monitors the `Client.send` channel, periodically sends heartbeats (`Ping`), and writes messages to the WebSocket connection.

## Getting Started

### Prerequisites

* Go (v1.25+)

### Configuration

This project is configured using **Environment Variables**. You can set the following parameters before running to override the default values:

* `PORT`: The port the service listens on (Default: `8080`).
* `ENVIRONMENT`: The running environment (Default: `development`).
* `ALLOWED_ORIGINS`: A comma-separated list of domains allowed for CORS (e.g., `http://localhost:5173,https://example.com`).

### Running Steps

1.  **Clone the Repository:**
    ```bash
    git clone https://github.com/hezhichaogt/hzchat-server
    cd hzchat-server
    ```
2.  **Set Environment Variables:**
    ```bash
    # Allow the frontend client address for cross-origin access
    export ALLOWED_ORIGINS="http://localhost:5173" 
    ```
3.  **Run the Service:**
    ```bash
    go run ./cmd/main.go
    ```
    > Once started, the API and WebSocket endpoints will be available by default at `http://localhost:8080`.

## 🤝 Contributing

This project is fully open-sourced under the **AGPL v3.0** license. We welcome and encourage the community to review the code and submit any issues or bug reports via GitHub Issues.