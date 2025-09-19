# Render 🖼️

**A high-performance Go server providing a real-time WebSocket API ⚡ for scraping unique images from Pinterest.**

Render is designed for developers and automated systems that require a reliable and efficient image-scraping solution. By leveraging a headless browser and an intelligent duplicate-detection system, Render delivers a continuous stream of fresh images while minimizing client-side complexity.

---

## ✨ Key Features

- **🧠 Intelligent Caching:** A background worker proactively scrapes images and stores them in an in-memory pool for instant delivery. A perceptual hashing algorithm (`dHash`) and a persistent `bbolt` database ensure that each client receives a unique set of images.
- **🛡️ Resilient & Reliable:** Features a **circuit breaker** to prevent repeated calls to failing services, **rate limiting** to avoid API bans, and **browser fingerprint randomization** to mimic real users.
- **⚡ High-Performance & Concurrent:** Built with Go and the `lxzan/gws` WebSocket library, Render is designed for high-concurrency and low-latency.
- **⚙️ Highly Configurable:** All key settings—from scraping behavior and API credentials to database cleanup—are managed through a simple `config.json` file.
- **🔄 Dynamic Queries:** A query rotation system automatically cycles through different search terms to ensure a diverse and continuous stream of fresh images.

---

## 🚀 Getting Started

### Prerequisites
- Go 1.18 or later
- A modern web browser supported by `chromedp` (e.g., Google Chrome, Microsoft Edge) for local development.

### Installation
1.  **Clone the repository:**
    ```bash
    git clone https://github.com/your-username/render.git
    cd render
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
      "Mozilla/5.0 (Windows NT 10.0; Win64; x64)...",
      "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7)..."
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
go build -o gopin.exe .
cd render-client
go build -o ../gopin-client.exe .
cd ..
```

### Running the Server
To start the server, run the executable from the project root:
```bash
./gopin.exe
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
    // You can save it to a file, for example:
    const fileName = `image_${Date.now()}.jpg`;
    fs.writeFileSync(fileName, data);
    console.log(`Saved image as ${fileName}`);
  } else {
    // It's a text message (like the pin ID)
    const message = data.toString();
    console.log(`Received message: ${message}`);
  }
});
---

## 🧪 Test Client

A simple Go-based test client is provided in the `render-client` directory to demonstrate how to connect to and interact with the Render server.

### Running the Test Client
Run the client executable from the project root with your desired flags.

- **Request 10 images with the default query:**
  ```bash
  ./gopin-client.exe --limit=10
  ```
- **Request 5 images with a custom query:**
  ```bash
  ./gopin-client.exe --query="neon city" --limit=5
  ```
- **Clear your client's history on the server:**
  ```bash
  ./gopin-client.exe --clear=true
  ```

### Client Flags
- `--query`: The search term for Pinterest.
- `--limit`: The number of unique images to download (default: 30).
- `--output`: The directory to save the images to (default: "output").
- `--server-name`: The client name for authentication (default: "my-discord-bot").
- `--password`: The password for authentication (default: "super-secret-password").
- `--clear`: If `true`, clears the client's image history on the server.