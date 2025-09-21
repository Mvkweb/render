package server

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"gopin/config"
	"gopin/database"
	"gopin/manager"
	"gopin/pkg/logger"
	"gopin/scraper"
	"net/http"
	"os"
	"time"

	"github.com/lxzan/gws"
)

const (
	PingInterval = 5 * time.Second
	PingWait     = 10 * time.Second
)

// Server holds the dependencies for the HTTP server.
type Server struct {
	router        *http.ServeMux
	upgrader      *gws.Upgrader
	config        *config.Config
	db            *database.DB
	scraper       *scraper.Scraper
	httpServer    *http.Server
	log           *logger.Logger
	scrapeManager *manager.ScrapeManager
	ctx           context.Context
}

// New creates a new Server.
func New(cfg *config.Config, log *logger.Logger) *Server {
	db, err := database.Open("data/render.db")
	if err != nil {
		log.Error("Failed to open database", "error", err)
		os.Exit(1)
	}

	scraperInstance, err := scraper.New(cfg.NumWorkers, log, cfg.Scraping.UserAgents)
	if err != nil {
		log.Error("Failed to create scraper", "error", err)
		os.Exit(1)
	}

	s := &Server{
		router:        http.NewServeMux(),
		config:        cfg,
		db:            db,
		scraper:       scraperInstance,
		log:           log,
		scrapeManager: manager.New(scraperInstance, db, log),
		ctx:           context.Background(), // Use a separate context for the server
	}

	upgrader := gws.NewUpgrader(s.newWsHandler(), &gws.ServerOption{
		ParallelEnabled:   true, // This is the key change
		Recovery:          gws.Recovery,
		PermessageDeflate: gws.PermessageDeflate{Enabled: true},
	})
	s.upgrader = upgrader

	s.routes()

	s.startCleanupTicker()

	return s
}

// Start runs the HTTP server.
func (s *Server) Start() error {
	s.httpServer = &http.Server{
		Addr:    fmt.Sprintf(":%s", s.config.Port),
		Handler: s.router,
	}

	s.log.Info("Server starting", "port", s.config.Port)
	if err := s.httpServer.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		return fmt.Errorf("failed to start server: %w", err)
	}
	return nil
}

// Shutdown gracefully shuts down the server.
func (s *Server) Shutdown(ctx context.Context) {
	s.log.Info("Shutting down server...")

	// Shutdown the http server
	if err := s.httpServer.Shutdown(ctx); err != nil {
		s.log.Error("HTTP server shutdown error", "error", err)
	}

	// Close the database connection
	if err := s.db.Close(); err != nil {
		s.log.Error("Database close error", "error", err)
	}

	// Close the browser manager
	s.scraper.Close()

	s.log.Info("Server shut down gracefully.")
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
			s.log.Error("Failed to upgrade connection", "error", err)
			return
		}
		socket.Session().Store("serverName", serverName)
		socket.ReadLoop() // This must be a blocking call
	}
}

// wsHandler implements the gws.Event interface.
type wsHandler struct {
	db            *database.DB
	log           *logger.Logger
	scrapeManager *manager.ScrapeManager
}

func (s *Server) newWsHandler() *wsHandler {
	return &wsHandler{
		db:            s.db,
		log:           s.log,
		scrapeManager: s.scrapeManager,
	}
}

// ScrapeRequest defines the structure for a client's scrape request.
type ScrapeRequest struct {
	Queries []string `json:"queries,omitempty"`
	Limit   int      `json:"limit,omitempty"`
	Command string   `json:"command,omitempty"`
}

func (c *wsHandler) OnOpen(socket *gws.Conn) {
	_ = socket.SetDeadline(time.Now().Add(PingInterval + PingWait))
}

func (c *wsHandler) OnClose(socket *gws.Conn, err error) {
	clientNameVal, _ := socket.Session().Load("serverName")
	clientName, _ := clientNameVal.(string)

	c.scrapeManager.Stop(clientName)
	c.log.Info("Client disconnected, stopping scrape pool", "client", clientName)

	c.log.Info("Socket closed", "remoteAddr", socket.RemoteAddr(), "error", err, "client", clientName)
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
		c.log.Warn("Invalid scrape request", "error", err, "remoteAddr", socket.RemoteAddr())
		return
	}

	clientNameVal, _ := socket.Session().Load("serverName")
	clientName, _ := clientNameVal.(string)

	if req.Command == "clear" {
		if err := c.db.ClearClientHistory(clientName); err != nil {
			c.log.Error("Failed to clear client history", "error", err, "client", clientName)
		} else {
			c.log.Info("Cleared client history", "client", clientName)
		}
		return
	}

	if len(req.Queries) == 0 {
		c.log.Warn("Received scrape request with no queries", "client", clientName)
		return
	}

	c.log.Info("Starting new scrape pool for client", "client", clientName, "queryCount", len(req.Queries), "limit", req.Limit)
	imageChan := c.scrapeManager.Start(clientName, req.Queries, req.Limit)

	// Start a goroutine to stream images to this client
	go func() {
		for img := range imageChan {
			// Check if the client has already seen this image
			seen, err := c.db.HasClientSeenImage(clientName, img.Hash)
			if err != nil {
				c.log.Error("Error checking if image was seen", "error", err, "client", clientName)
				continue
			}
			if seen {
				continue // Skip seen images
			}

			// Send the raw image data
			if err := socket.WriteMessage(gws.OpcodeBinary, img.Data); err != nil {
				c.log.Error("Error sending image to client", "error", err, "client", clientName)
				return // Stop if we can't send
			}

			// Let the client know the pin ID
			socket.WriteMessage(gws.OpcodeText, []byte(fmt.Sprintf("pin:%s", img.ID)))

			// Mark the image as seen for this client
			if err := c.db.MarkImageAsSeen(clientName, img.Hash); err != nil {
				c.log.Error("Error marking image as seen", "error", err, "client", clientName)
			}
		}
	}()
}

// startCleanupTicker starts a goroutine that periodically cleans up old entries from the database.
func (s *Server) startCleanupTicker() {
	cleanupInterval, err := time.ParseDuration(s.config.Database.CleanupInterval)
	if err != nil {
		s.log.Error("Invalid database cleanup interval in config.json", "error", err)
		return
	}

	maxAge, err := time.ParseDuration(s.config.Database.MaxAge)
	if err != nil {
		s.log.Error("Invalid database max age in config.json", "error", err)
		return
	}

	ticker := time.NewTicker(cleanupInterval)
	go func() {
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				s.log.Info("Running database cleanup...")
				if err := s.db.CleanupOldEntries(maxAge); err != nil {
					s.log.Error("Database cleanup failed", "error", err)
				} else {
					s.log.Info("Database cleanup finished.")
				}
			case <-s.ctx.Done():
				return
			}
		}
	}()
}
