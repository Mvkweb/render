# gopin

`gopin` is a lightweight, concurrent Pinterest image scraper written in Go. It uses a headless browser to scrape search results and a persistent database to avoid re-downloading images.

## Features

- **Infinite Scrolling:** Automatically scrolls down the search results page to find all available images.
- **Persistent De-duplication:** Uses a `bbolt` database to store the IDs of downloaded pins and the hashes of downloaded images, preventing re-downloads across multiple sessions.
- **Concurrent Downloads:** Uses a worker pool to download images concurrently.
- **Lightweight:** Has minimal dependencies.

## Usage

```bash
go run main.go [options]
```

### Options

- `-query`: The search query (default: "art").
- `-limit`: The maximum number of images to download (default: 50).
- `-output`: The directory to save images to (default: "output").
- `-workers`: The number of concurrent download workers (default: 10).
- `-db`: The path to the database file (default: "pinterest.db").