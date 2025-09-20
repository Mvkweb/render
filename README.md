<div align="center">
  <h1>Render 🖼️</h1>
  <p><strong>A high-performance, standalone Go server that scrapes unique, high-quality images from Pinterest and delivers them via a real-time WebSocket API.</strong></p>
</div>

<div align="center">

[![Go Version](https://img.shields.io/badge/Go-1.25+-00ADD8?style=for-the-badge&logo=go&logoColor=white)](https://go.dev/)
[![License](https://img.shields.io/badge/License-MIT-green.svg?style=for-the-badge)](https://opensource.org/licenses/MIT)
[![GitHub Stars](https://img.shields.io/github/stars/your-username/Render.svg?style=for-the-badge&logo=github)](https://github.com/your-username/Render/stargazers)

</div>

<hr>

## Introduction

**Render** is an efficient and resilient image scraping solution for developers and automated systems. It acts as a smart proxy to Pinterest, leveraging headless browser automation and intelligent deduplication to provide a continuous stream of fresh images through a simple WebSocket API, abstracting away the complexities of web scraping.

## How It Works: Under the Hood

Render is not a simple scraper. It employs several advanced techniques to ensure efficiency, reliability, and a high-quality output.

-   **Headless Browser & Network Interception**: Instead of parsing the messy HTML DOM, Render launches a headless Microsoft Edge instance using `chromedp`. It navigates to Pinterest and programmatically scrolls, but its real power comes from intercepting the background network requests (XHR) made by the Pinterest frontend. It specifically targets the `BaseSearchResource` API endpoint, capturing the clean, structured JSON data that contains image URLs and metadata. This is significantly faster and more reliable than traditional screen scraping.

-   **Advanced Deduplication**: To ensure clients always receive unique content, Render uses a two-layer approach:
    1.  **Perceptual Hashing (`dHash`)**: When an image is downloaded, a "perceptual hash" is generated. Unlike cryptographic hashes (like SHA-256), `dHash` can identify images that are visually similar, even if they have minor differences in resolution, compression, or watermarking. This prevents near-duplicates from entering the pool.
    2.  **Persistent Client History**: An embedded `bbolt` key-value database tracks every image hash sent to each unique client. This guarantees that a client will never receive the same image twice, even across server restarts.

-   **High-Concurrency Architecture**:
    *   **Worker Pools**: Image downloading and processing are handled by a configurable number of worker goroutines, allowing dozens of images to be fetched and hashed in parallel.
    *   **Asynchronous WebSocket**: The server is built on `lxzan/gws`, a high-performance WebSocket library configured for parallel message handling, ensuring the server remains responsive even with many connected clients.

-   **Resilience and Evasion**:
    *   **Circuit Breaker**: A built-in circuit breaker monitors the scraping process. If Pinterest becomes unresponsive or starts throwing errors, the breaker will "trip," temporarily halting scraping attempts to avoid hammering a failing service and allowing it time to recover.
    *   **Human-like Rate Limiting**: The scraper introduces randomized delays between actions (like scrolling) to mimic human browsing behavior, making it harder for anti-bot systems to detect.
    *   **Fingerprint Evasion**: The headless browser's fingerprint is randomized on each launch, using a variety of user agents, window sizes, and specific `chromedp` flags (`disable-blink-features`, `excludeSwitches`) to mask its automated nature.

-   **Intelligent Caching & Query Rotation**:
    *   **In-Memory Pool**: Freshly scraped images are held in a large, in-memory pool for near-instant delivery to clients.
    *   **Background Refresh**: A background task periodically runs, picking a new query from the `config.json` list and refreshing the image pool to ensure a continuous and diverse supply of content.

## Key Features

- **🧠 Intelligent Caching:** Proactive background scraping with an in-memory pool for instant delivery.
- **🚫 Advanced Duplicate Detection:** Perceptual hashing (`dHash`) combined with a persistent `bbolt` database to ensure unique images for every client.
- **🛡️ Resilient & Reliable:** Features a **circuit breaker**, **rate limiting**, and **browser fingerprint randomization** to ensure robust, long-term operation.
- **⚡ High-Performance & Concurrent:** Built with Go and the `lxzan/gws` WebSocket library for high-concurrency and low-latency.
- **⚙️ Highly Configurable:** All key settings—from scraping behavior and API credentials to database cleanup—are managed through a simple `config.json` file.
- **🔄 Dynamic Queries:** A query rotation system automatically cycles through different search terms to ensure a diverse and continuous stream of fresh images.

---

## 🚀 Getting Started

### Prerequisites
- Go 1.18 or later
- A modern web browser supported by `chromedp` (e.g., Google Chrome, Microsoft Edge). The scraper is hardcoded to look for Microsoft Edge on Windows.

### Installation
1.  **Clone the repository:**
    ```bash
    git clone https://github.com/your-username/Render.git
    cd Render
    ```
2.  **Install dependencies:**
    ```bash
    go mod tidy
    ```

### Configuration
The server is configured via `config.json`. The default file includes sensible settings for scraping, database management, and more.

**`config.json`**
```json
{
  "port": "8080",
  "credentials": {
    "my-discord-bot": "super-secret-password"
  },
  "numWorkers": 10,
  "scraping": {
    "minDelay": "5s",
    "maxDelay": "15s",
    "poolSize": 200,
    "refreshInterval": "30m",
    "queries": [
      "dark aesthetic discord pfp",
      "anime discord avatar",
      "gothic profile picture"
    ],
    "userAgents": [
      "Mozilla/5.0 (Windows NT 10.0; Win64; x64)..."
    ]
  },
  "database": {
    "cleanupInterval": "24h",
    "maxAge": "30d"
  }
}
```

### Building the Application
To build the server and client executables, run:
```bash
go build -o build/Render-server ./cmd/server
go build -o build/Render-client ./cmd/client
```

### Running the Server
To start the server, run the executable from the project root:
```bash
./build/Render-server
```
The server will start on the port specified in your `config.json`.

---

## 🔌 API Usage

The server exposes a single WebSocket endpoint at `/scrape`.

### 1. Connection
To connect, establish a WebSocket connection to `ws://<server-address>:<port>/scrape`. You must provide your credentials in the HTTP headers of the upgrade request:

- `X-Server-Name`: Your client's name (e.g., `my-discord-bot`)
- `X-Password`: Your client's password (e.g., `super-secret-password`)

**Example (JavaScript):**
```javascript
const WebSocket = require('ws');

const options = {
  headers: {
    'X-Server-Name': 'my-discord-bot',
    'X-Password': 'super-secret-password'
  }
};

const ws = new WebSocket('ws://localhost:8080/scrape', options);

ws.on('open', function open() {
  console.log('Connected to Render! 🚀');
  // Now you can send scrape requests
});

ws.on('error', function error(err) {
    console.error('WebSocket error:', err);
});
```

### 2. Requesting Images
Once connected, send a JSON message to request images:

**`request.json`**
```json
{
  "query": "cyberpunk art",
  "limit": 5
}
```
*Note: The `query` field is not used for image selection, as images are served from the pre-filled, rotating pool to ensure uniqueness and fast delivery.*

**Example (JavaScript):**
```javascript
ws.on('open', function open() {
  const request = {
    query: 'cyberpunk art',
    limit: 5
  };
  ws.send(JSON.stringify(request));
});
```

### 3. Receiving Images
The server will stream back the requested number of unique images. The data will arrive in two forms:
- **Binary Messages:** The raw image data (`image/jpeg`, `image/png`, etc.).
- **Text Messages:** The corresponding Pinterest pin ID, in the format `pin:<id>`.

**Example (JavaScript):**
```javascript
const fs = require('fs');

ws.on('message', function incoming(data) {
  if (Buffer.isBuffer(data)) {
    // It's an image!
    console.log('Received image data! 🖼️');
    const fileName = `image_${Date.now()}.jpg`;
    fs.writeFileSync(fileName, data);
    console.log(`Saved image as ${fileName}`);
  } else {
    // It's a text message (like the pin ID)
    const message = data.toString();
    console.log(`Received message: ${message}`);
  }
});
```
---

## 🧪 Test Client

A simple Go-based test client is provided in the `cmd/client` directory to demonstrate how to connect to and interact with the server.

### Running the Test Client
Run the client executable with your desired flags.

- **Request 10 images with the default query:**
  ```bash
  ./build/Render-client --limit=10
  ```
- **Request 5 images with a custom query:**
  ```bash
  ./build/Render-client --query="neon city" --limit=5
  ```
- **Clear your client's history on the server:**
  ```bash
  ./build/Render-client --clear=true
  ```

### Client Flags
- `--query`: The search term for Pinterest.
- `--limit`: The number of unique images to download (default: 30).
- `--output`: The directory to save the images to (default: "output").
- `--server-name`: The client name for authentication (default: "my-discord-bot").
- `--password`: The password for authentication (default: "super-secret-password").
- `--clear`: If `true`, clears the client's image history on the server.