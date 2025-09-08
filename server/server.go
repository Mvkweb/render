package server

import (
	"encoding/json"
	"fmt"
	"gopin/config"
	"gopin/database"
	"gopin/scraper"
	"log"
	"net/http"
	"strconv"
	"time"

	"github.com/lxzan/gws"
)

const (
	PingInterval = 5 * time.Second
	PingWait     = 10 * time.Second
)

// Server holds the dependencies for the HTTP server.
type Server struct {
	router   *http.ServeMux
	upgrader *gws.Upgrader
	config   *config.Config
	db       *database.DB
	scraper  *scraper.Scraper
}

// New creates a new Server.
func New(cfg *config.Config) *Server {
	db, err := database.Open("gopin.db")
	if err != nil {
		log.Fatalf("Failed to open database: %v", err)
	}

	scraper := scraper.New()

	s := &Server{
		router:  http.NewServeMux(),
		config:  cfg,
		db:      db,
		scraper: scraper,
	}

	upgrader := gws.NewUpgrader(s.newWsHandler(), &gws.ServerOption{
		ParallelEnabled:   true,
		Recovery:          gws.Recovery,
		PermessageDeflate: gws.PermessageDeflate{Enabled: true},
	})
	s.upgrader = upgrader

	s.routes()
	return s
}

// Start runs the HTTP server.
func (s *Server) Start() {
	log.Printf("Server starting on port %s", s.config.Port)
	if err := http.ListenAndServe(fmt.Sprintf(":%s", s.config.Port), s.router); err != nil {
		log.Fatalf("Failed to start server: %v", err)
	}
}

// routes registers the HTTP handlers for the server.
func (s *Server) routes() {
	s.router.HandleFunc("/", s.handleIndex())
	s.router.HandleFunc("/scrape", s.authMiddleware(s.handleScrape()))
}

// handleIndex is a simple handler for the root endpoint.
func (s *Server) handleIndex() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintln(w, "Image scraper server is running.")
	}
}

// authMiddleware checks for valid credentials before allowing access.
func (s *Server) authMiddleware(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		serverName := r.Header.Get("X-Server-Name")
		password := r.Header.Get("X-Password")

		expectedPassword, ok := s.config.Credentials[serverName]
		if !ok || expectedPassword != password {
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}

		next.ServeHTTP(w, r)
	}
}

// handleScrape handles the websocket connection for scraping.
func (s *Server) handleScrape() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// Store the server name in the session context for later use
		serverName := r.Header.Get("X-Server-Name")

		socket, err := s.upgrader.Upgrade(w, r)
		if err != nil {
			log.Printf("Failed to upgrade connection: %v", err)
			return
		}
		socket.Session().Store("serverName", serverName)
		go func() {
			socket.ReadLoop() // Blocking prevents the context from being GC.
		}()
	}
}

// wsHandler implements the gws.Event interface.
type wsHandler struct {
	db      *database.DB
	scraper *scraper.Scraper
}

func (s *Server) newWsHandler() *wsHandler {
	return &wsHandler{
		db:      s.db,
		scraper: s.scraper,
	}
}

// ScrapeRequest defines the structure for a client's scrape request.
type ScrapeRequest struct {
	Query string `json:"query"`
	Limit int    `json:"limit"`
}

func (c *wsHandler) OnOpen(socket *gws.Conn) {
	_ = socket.SetDeadline(time.Now().Add(PingInterval + PingWait))
}

func (c *wsHandler) OnClose(socket *gws.Conn, err error) {
	log.Printf("Socket closed: %v", err)
}

func (c *wsHandler) OnPing(socket *gws.Conn, payload []byte) {
	_ = socket.SetDeadline(time.Now().Add(PingInterval + PingWait))
	_ = socket.WritePong(nil)
}

func (c *wsHandler) OnPong(socket *gws.Conn, payload []byte) {}

func (c *wsHandler) OnMessage(socket *gws.Conn, message *gws.Message) {
	defer message.Close()

	var req ScrapeRequest
	if err := json.Unmarshal(message.Bytes(), &req); err != nil {
		log.Printf("Invalid scrape request: %v", err)
		return
	}

	serverName, _ := socket.Session().Load("serverName")
	clientName, _ := serverName.(string)

	log.Printf("Starting scrape for client '%s', query: '%s', limit: %d", clientName, req.Query, req.Limit)

	imageChan, err := c.scraper.Scrape(req.Query, req.Limit*5) // Fetch more to account for duplicates
	if err != nil {
		log.Printf("Failed to start scrape: %v", err)
		return
	}

	count := 0
	for img := range imageChan {
		if count >= req.Limit {
			break
		}

		seen, err := c.db.HasClientSeenImage(clientName, img.Hash)
		if err != nil {
			log.Printf("Error checking if image was seen: %v", err)
			continue
		}
		if seen {
			continue
		}

		// Send the raw image data
		if err := socket.WriteMessage(gws.OpcodeBinary, img.Data); err != nil {
			log.Printf("Error sending image to client: %v", err)
			return // Stop if we can't send
		}

		// Let the client know the pin ID
		socket.WriteMessage(gws.OpcodeText, []byte(fmt.Sprintf("pin:%s", img.ID)))

		if err := c.db.MarkImageAsSeen(clientName, img.Hash); err != nil {
			log.Printf("Error marking image as seen: %v", err)
		}

		// Also mark the pin ID as seen to avoid re-downloading it from pinterest
		pinHash, _ := strconv.ParseUint(img.ID, 10, 64)
		if err := c.db.MarkImageAsSeen(clientName, pinHash); err != nil {
			log.Printf("Error marking pin as seen: %v", err)
		}

		count++
	}

	log.Printf("Finished sending %d images to client '%s'", count, clientName)
}
