package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/lxzan/gws"
)

const (
	PingInterval = 5 * time.Second
)

// ScrapeRequest defines the structure for a client's scrape request.
type ScrapeRequest struct {
	Query   string `json:"query"`
	Limit   int    `json:"limit"`
	Command string `json:"command,omitempty"`
}

type wsHandler struct {
	outputDir string
	limit     int
}

func (c *wsHandler) OnOpen(socket *gws.Conn) {
	socket.Session().Store("image_count", 0)
	socket.Session().Store("scrape_complete", false)
	log.Println("Connected to Render! 🚀")
}

func (c *wsHandler) OnClose(socket *gws.Conn, err error) {
	log.Printf("Socket closed: %v", err)
}

func (c *wsHandler) OnPing(socket *gws.Conn, payload []byte) {}
func (c *wsHandler) OnPong(socket *gws.Conn, payload []byte) {}
func (c *wsHandler) OnMessage(socket *gws.Conn, message *gws.Message) {
	defer message.Close()

	if message.Opcode == gws.OpcodeBinary {
		// It's an image!
		log.Println("Received image data! 🖼️")
		currentCount, _ := socket.Session().Load("image_count")
		count := currentCount.(int)

		// You can save it to a file, for example:
		fileName := fmt.Sprintf("image_%d.jpg", count)
		filePath := filepath.Join(c.outputDir, fileName)
		if err := os.WriteFile(filePath, message.Bytes(), 0644); err != nil {
			log.Printf("Failed to save image: %v", err)
		} else {
			log.Printf("Saved image as %s", filePath)
		}
		count++
		socket.Session().Store("image_count", count)

		// Check if we are done
		scrapeComplete, _ := socket.Session().Load("scrape_complete")
		if scrapeComplete.(bool) && count >= c.limit {
			socket.WriteClose(1000, []byte("work complete"))
		}
	} else {
		// It's a text message (like the pin ID)
		msgStr := string(message.Bytes())
		log.Printf("Received message: %s", msgStr)
		if msgStr == "scrape_complete" {
			socket.Session().Store("scrape_complete", true)
			currentCount, _ := socket.Session().Load("image_count")
			if currentCount.(int) >= c.limit {
				socket.WriteClose(1000, []byte("work complete"))
			}
		}
	}
}

func main() {
	query := flag.String("query", "dark pfp discord girl", "The search query.")
	limit := flag.Int("limit", 30, "The maximum number of images to download.")
	outputDir := flag.String("output", "output", "The directory to save images to.")
	clear := flag.Bool("clear", false, "Clear the client's history on the server.")
	serverName := flag.String("server-name", "my-discord-bot", "The server name for authentication.")
	password := flag.String("password", "super-secret-password", "The password for authentication.")
	flag.Parse()

	// Clean and recreate the output directory
	if err := os.RemoveAll(*outputDir); err != nil {
		log.Fatalf("Failed to clean output directory: %v", err)
	}
	if err := os.MkdirAll(*outputDir, 0755); err != nil {
		log.Fatalf("Failed to create output directory: %v", err)
	}

	headers := http.Header{}
	headers.Set("X-Server-Name", *serverName)
	headers.Set("X-Password", *password)

	handler := &wsHandler{outputDir: *outputDir, limit: *limit}

	socket, _, err := gws.NewClient(handler, &gws.ClientOption{
		Addr:          "ws://localhost:8080/scrape",
		RequestHeader: headers,
	})
	if err != nil {
		log.Fatalf("Failed to connect: %v", err)
	}

	// Send the scrape request
	req := ScrapeRequest{
		Query: *query,
		Limit: *limit,
	}
	if *clear {
		req.Command = "clear"
	}
	reqBytes, _ := json.Marshal(req)
	if err := socket.WriteMessage(gws.OpcodeText, reqBytes); err != nil {
		log.Printf("Failed to send request: %v", err)
	}

	// Start a goroutine to send pings
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go func() {
		ticker := time.NewTicker(PingInterval)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				if err := socket.WritePing(nil); err != nil {
					log.Printf("Failed to send ping: %v", err)
					return
				}
			case <-ctx.Done():
				return
			}
		}
	}()

	socket.ReadLoop()
}
