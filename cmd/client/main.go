package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"sync/atomic"
	"syscall"
	"time"

	"github.com/lxzan/gws"
)

const (
	PingInterval = 5 * time.Second
)

// ScrapeRequest defines the structure for a client's scrape request.
type ScrapeRequest struct {
	Queries []string `json:"queries,omitempty"`
	Command string   `json:"command,omitempty"`
}

type wsHandler struct {
	outputDir  string
	imageCount int64
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
		count := atomic.AddInt64(&c.imageCount, 1)
		fileName := fmt.Sprintf("image_%d.jpg", count)
		filePath := filepath.Join(c.outputDir, fileName)
		if err := os.WriteFile(filePath, message.Bytes(), 0644); err != nil {
			log.Printf("Failed to save image: %v", err)
		} else {
			log.Printf("Saved image as %s", filePath)
		}
	} else {
		log.Printf("Received message: %s", string(message.Bytes()))
	}
}

func main() {
	queriesFile := flag.String("queries", "queries.json", "Path to the JSON file containing a list of queries.")
	outputDir := flag.String("output", "output", "The directory to save images to.")
	clear := flag.Bool("clear", false, "Clear the client's history on the server.")
	serverName := flag.String("server-name", "my-discord-bot", "The server name for authentication.")
	password := flag.String("password", "super-secret-password", "The password for authentication.")
	flag.Parse()

	if err := os.MkdirAll(*outputDir, 0755); err != nil {
		log.Fatalf("Failed to create output directory: %v", err)
	}

	// Load queries from the file
	queries, err := loadQueries(*queriesFile)
	if err != nil {
		log.Fatalf("Failed to load queries: %v", err)
	}

	headers := http.Header{}
	headers.Set("X-Server-Name", *serverName)
	headers.Set("X-Password", *password)

	handler := &wsHandler{outputDir: *outputDir}

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	socket, _, err := gws.NewClient(handler, &gws.ClientOption{
		Addr:          "ws://localhost:8080/scrape",
		RequestHeader: headers,
		PermessageDeflate: gws.PermessageDeflate{
			Enabled: true,
		},
	})
	if err != nil {
		log.Fatalf("Failed to connect: %v", err)
	}

	go func() {
		// Send initial clear request if specified
		if *clear {
			req := ScrapeRequest{Command: "clear"}
			reqBytes, _ := json.Marshal(req)
			if err := socket.WriteMessage(gws.OpcodeText, reqBytes); err != nil {
				log.Printf("Failed to send clear request: %v", err)
				return
			}
			log.Println("Sent clear history request.")
			time.Sleep(1 * time.Second) // Give server a moment to process
		}

		// Send the full list of queries to the server
		req := ScrapeRequest{Queries: queries}
		reqBytes, _ := json.Marshal(req)
		log.Println("Sending query list to server...")
		if err := socket.WriteMessage(gws.OpcodeText, reqBytes); err != nil {
			log.Printf("Failed to send query list: %v", err)
			return
		}
	}()

	// Start a goroutine to send pings
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

	go func() {
		<-ctx.Done()
		log.Println("Shutting down client...")
		socket.WriteClose(1000, []byte("client shutdown"))
	}()

	socket.ReadLoop()
}

func loadQueries(path string) ([]string, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	var queries []string
	decoder := json.NewDecoder(file)
	if err := decoder.Decode(&queries); err != nil {
		return nil, err
	}
	return queries, nil
}
