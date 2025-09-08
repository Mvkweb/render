# Render 🖼️

**A high-performance Go server providing a real-time WebSocket API ⚡ for scraping unique images from Pinterest.**

Render is designed for developers and automated systems that require a reliable and efficient image-scraping solution. By leveraging a headless browser and an intelligent duplicate-detection system, Render delivers a continuous stream of fresh images while minimizing client-side complexity.

---

## ✨ Key Features

- **🧠 Intelligent Uniqueness:** A perceptual hashing algorithm (`dHash`) and a persistent `bbolt` database ensure that each client receives a unique set of images for every query.
- **⚡ High-Performance & Concurrent:** Built with Go and the `lxzan/gws` WebSocket library, Render is designed for high-concurrency and low-latency.
- **🛡️ Robust Scraping Engine:** By using a headless browser (`chromedp`), Render accurately simulates a real user, making it resilient to website changes.
- **🔒 Simple and Secure API:** The server exposes a single WebSocket endpoint, protected by a straightforward, header-based authentication scheme.
- **⚙️ Configurable:** All key settings, including the server port and client credentials, are managed through a simple `config.json` file.

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
The server is configured via a `config.json` file. Create one in the root of the project:

**`config.json`**
```json
{
  "port": "8080",
  "credentials": {
    "my-discord-bot": "super-secret-password",
    "another-client": "password123"
  }
}
```

### Running the Server
To start the server, run:
```bash
go run main.go
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