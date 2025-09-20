package server

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"gopin/config"
	"gopin/database"
	"gopin/pkg/logger"
	"gopin/scraper"
	"net/http"
	"os"
	"sync"
	"time"

	"github.com/lxzan/gws"
)

const (
	PingInterval = 5 * time.Second
	PingWait     = 10 * time.Second
)

// Server holds the dependencies for the HTTP server.
type Server struct {
	router       *http.ServeMux
	upgrader     *gws.Upgrader
	config       *config.Config
	db           *database.DB
	scraper      *scraper.Scraper
	httpServer   *http.Server
	log          *logger.Logger
	clientLocks  map[string]*sync.Mutex
	locksMutex   sync.Mutex
	imagePool    *ImagePool
	queryManager *scraper.QueryManager
	ctx          context.Context
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
		router:      http.NewServeMux(),
		config:      cfg,
		db:          db,
		scraper:     scraperInstance,
		log:         log,
		clientLocks: make(map[string]*sync.Mutex),
		imagePool:   NewImagePool(cfg.Scraping.PoolSize),
		queryManager: scraper.NewQueryManager(cfg.Scraping.Queries, []string{
			"aesthetic", "dark", "pastel", "vintage",
			"grunge", "minimal", "neon", "retro",
		}),
		ctx: context.Background(), // Use a separate context for the server
	}

	upgrader := gws.NewUpgrader(s.newWsHandler(), &gws.ServerOption{
		ParallelEnabled:   true, // This is the key change
		Recovery:          gws.Recovery,
		PermessageDeflate: gws.PermessageDeflate{Enabled: true},
	})
	s.upgrader = upgrader

	s.routes()

	// Perform an initial scrape to populate the pool
	go s.refreshImagePool()

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
	db          *database.DB
	scraper     *scraper.Scraper
	log         *logger.Logger
	locksMutex  *sync.Mutex
	clientLocks map[string]*sync.Mutex
	imagePool   *ImagePool
}

func (s *Server) newWsHandler() *wsHandler {
	return &wsHandler{
		db:          s.db,
		scraper:     s.scraper,
		log:         s.log,
		clientLocks: s.clientLocks,
		locksMutex:  &s.locksMutex,
		imagePool:   s.imagePool,
	}
}

// ScrapeRequest defines the structure for a client's scrape request.
type ScrapeRequest struct {
	Query   string `json:"query"`
	Limit   int    `json:"limit"`
	Command string `json:"command,omitempty"`
}

func (c *wsHandler) OnOpen(socket *gws.Conn) {
	_ = socket.SetDeadline(time.Now().Add(PingInterval + PingWait))
}

func (c *wsHandler) OnClose(socket *gws.Conn, err error) {
	c.log.Info("Socket closed", "remoteAddr", socket.RemoteAddr(), "error", err)
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

	serverName, _ := socket.Session().Load("serverName")
	clientName, _ := serverName.(string)

	if req.Command == "clear" {
		if err := c.db.ClearClientHistory(clientName); err != nil {
			c.log.Error("Failed to clear client history", "error", err, "client", clientName)
		} else {
			c.log.Info("Cleared client history", "client", clientName)
		}
		return
	}

	// Acquire lock for the client
	c.locksMutex.Lock()
	if _, ok := c.clientLocks[clientName]; !ok {
		c.clientLocks[clientName] = &sync.Mutex{}
	}
	mu := c.clientLocks[clientName]
	c.locksMutex.Unlock()

	if !mu.TryLock() {
		c.log.Warn("Scrape already in progress for client", "client", clientName)
		return
	}

	// Run the image sending in a goroutine to avoid blocking the read loop
	go func() {
		defer mu.Unlock() // Release lock when done

		c.log.Info("Starting image delivery", "client", clientName, "limit", req.Limit)

		for i := 0; i < req.Limit; i++ {
			img, err := c.imagePool.GetRandomUnseenImage(c.db, clientName)
			if err != nil {
				c.log.Warn("Could not get unseen image from pool, waiting for next refresh", "error", err, "client", clientName)
				socket.WriteMessage(gws.OpcodeText, []byte("no_unseen_images_in_pool"))
				time.Sleep(5 * time.Second) // Wait before retrying
				i--                         // Retry the same image index
				continue
			}

			// Send the raw image data
			if err := socket.WriteMessage(gws.OpcodeBinary, img.Data); err != nil {
				c.log.Error("Error sending image to client", "error", err)
				return // Stop if we can't send
			}

			// Let the client know the pin ID
			socket.WriteMessage(gws.OpcodeText, []byte(fmt.Sprintf("pin:%s", img.ID)))

			if err := c.db.MarkImageAsSeen(clientName, img.Hash); err != nil {
				c.log.Error("Error marking image as seen", "error", err)
			}
		}

		c.log.Info("Finished sending images", "client", clientName, "count", req.Limit)
		// Optionally, send a completion message
		socket.WriteMessage(gws.OpcodeText, []byte("scrape_complete"))
	}()
}

// StartBackgroundScraper starts a ticker to periodically refresh the image pool.
func (s *Server) StartBackgroundScraper(interval time.Duration) {
	ticker := time.NewTicker(interval)
	go func() {
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				s.refreshImagePool()
			case <-s.ctx.Done():
				return
			}
		}
	}()
}

// refreshImagePool gets a new query and scrapes Pinterest for fresh images.
func (s *Server) refreshImagePool() {
	query := s.queryManager.GetNextQuery()
	s.log.Info("Background scraping", "query", query)

	imageChan, err := s.scraper.Scrape(query, 50) // Get 50 fresh images
	if err != nil {
		s.log.Error("Background scrape failed", "error", err)
		return
	}

	var images []scraper.ScrapedImage
	for img := range imageChan {
		images = append(images, img)
	}

	s.imagePool.AddImages(images)
	s.log.Info("Image pool refreshed", "newImageCount", len(images))
}
