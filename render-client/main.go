package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"

	"github.com/lxzan/gws"
)

// ScrapeRequest defines the structure for a client's scrape request.
type ScrapeRequest struct {
	Query string `json:"query"`
	Limit int    `json:"limit"`
}

type wsHandler struct {
	outputDir string
}

func (c *wsHandler) OnOpen(socket *gws.Conn) {
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
		// You can save it to a file, for example:
		fileName := fmt.Sprintf("image_%d.jpg", currentCount)
		filePath := filepath.Join(c.outputDir, fileName)
		if err := os.WriteFile(filePath, message.Bytes(), 0644); err != nil {
			log.Printf("Failed to save image: %v", err)
		} else {
			log.Printf("Saved image as %s", filePath)
		}
		socket.Session().Store("image_count", currentCount.(int)+1)
	} else {
		// It's a text message (like the pin ID)
		msgStr := string(message.Bytes())
		log.Printf("Received message: %s", msgStr)
		if msgStr == "scrape_complete" {
			socket.WriteClose(1000, []byte("work complete"))
		}
	}
}

func main() {
	query := flag.String("query", "art", "The search query.")
	limit := flag.Int("limit", 5, "The maximum number of images to download.")
	outputDir := flag.String("output", "output", "The directory to save images to.")
	serverName := flag.String("server-name", "my-discord-bot", "The server name for authentication.")
	password := flag.String("password", "super-secret-password", "The password for authentication.")
	flag.Parse()

	if err := os.MkdirAll(*outputDir, 0755); err != nil {
		log.Fatalf("Failed to create output directory: %v", err)
	}

	headers := http.Header{}
	headers.Set("X-Server-Name", *serverName)
	headers.Set("X-Password", *password)

	handler := &wsHandler{outputDir: *outputDir}

	socket, _, err := gws.NewClient(handler, &gws.ClientOption{
		Addr:          "ws://localhost:8080/scrape",
		RequestHeader: headers,
	})
	if err != nil {
		log.Fatalf("Failed to connect: %v", err)
	}
	socket.Session().Store("image_count", 0)

	// Send the scrape request
	req := ScrapeRequest{
		Query: *query,
		Limit: *limit,
	}
	reqBytes, _ := json.Marshal(req)
	if err := socket.WriteMessage(gws.OpcodeText, reqBytes); err != nil {
		log.Printf("Failed to send request: %v", err)
	}

	socket.ReadLoop()
}
