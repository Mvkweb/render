# Go Image Scraper Server

This project is a high-performance, standalone image scraping server written in Go. It provides a WebSocket-based API to request images from Pinterest, ensuring that clients receive unique images for each query.

## Architecture

The server is built with a focus on performance and low client-side load. It uses the `lxzan/gws` library for efficient WebSocket communication and `bbolt` for a simple, file-based database to track image uniqueness per client.

The core components are:
- **Web Server**: A standard `net/http` server that handles WebSocket upgrade requests.
- **WebSocket Handler**: Manages the client connection, processes scrape requests, and streams back image data.
- **Authentication**: A middleware that protects the scrape endpoint with a simple servername/password scheme.
- **Scraper Service**: A headless browser-based scraper (using `chromedp`) that fetches image information from Pinterest.
- **Uniqueness Service**: A `bbolt`-backed service that tracks the hashes of images sent to each client, preventing duplicate images from being sent.

## Getting Started

### Prerequisites
- Go 1.18 or later
- A modern web browser supported by `chromedp` (e.g., Google Chrome, Microsoft Edge)

### Installation
1. Clone the repository:
   ```bash
   git clone <repository-url>
   cd gopin
   ```
2. Install the dependencies:
   ```bash
   go mod tidy
   ```

### Configuration
The server is configured via a `config.json` file in the root of the project. A sample configuration is provided below:

```json
{
  "port": "8080",
  "credentials": {
    "my-discord-bot": "super-secret-password",
    "another-client": "password123"
  }
}
```

- `port`: The port the server will run on.
- `credentials`: A map of `servername: password` pairs for API authentication.

### Running the Server
To start the server, run:
```bash
go run main.go
```
The server will start on the port specified in your `config.json`.

## API Usage

The server exposes a single WebSocket endpoint at `/scrape`.

### Connection
To connect to the server, establish a WebSocket connection to `ws://<server-address>:<port>/scrape`. You must provide your credentials in the HTTP headers of the upgrade request:

- `X-Server-Name`: Your client's name (e.g., `my-discord-bot`)
- `X-Password`: Your client's password (e.g., `super-secret-password`)

### Requesting Images
Once connected, you can send a JSON message to request images:

```json
{
  "query": "your search query",
  "limit": 10
}
```

- `query`: The search term for Pinterest.
- `limit`: The number of unique images you want to receive.

### Receiving Images
The server will stream back the requested number of unique images as raw binary data over the WebSocket. Each image is followed by a text message with the pin ID, in the format `pin:<id>`.