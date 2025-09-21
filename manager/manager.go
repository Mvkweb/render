package manager

import (
	"context"
	"gopin/database"
	"gopin/pkg/logger"
	"gopin/query"
	"gopin/scraper"
	"sync"
)

// ScrapeManager manages the lifecycle of scraping jobs.
type ScrapeManager struct {
	scraper *scraper.Scraper
	db      *database.DB
	log     *logger.Logger
	jobs    map[string]*ScrapeJob
	mu      sync.Mutex
}

// ScrapeJob represents an active scraping job.
type ScrapeJob struct {
	clientName   string
	queryManager *query.Manager
	imageChan    chan scraper.ScrapedImage
	log          *logger.Logger
	limit        int
	scraper      *scraper.Scraper
	ctx          context.Context
	cancel       context.CancelFunc
	wg           sync.WaitGroup
}

// New creates a new ScrapeManager.
func New(scraper *scraper.Scraper, db *database.DB, log *logger.Logger) *ScrapeManager {
	return &ScrapeManager{
		scraper: scraper,
		db:      db,
		log:     log,
		jobs:    make(map[string]*ScrapeJob),
	}
}

// Start creates and starts a new scraping job for a client.
func (m *ScrapeManager) Start(clientName string, queries []string, limit int) <-chan scraper.ScrapedImage {
	m.mu.Lock()
	defer m.mu.Unlock()

	if job, exists := m.jobs[clientName]; exists {
		job.Stop()
	}

	ctx, cancel := context.WithCancel(context.Background())
	job := &ScrapeJob{
		clientName:   clientName,
		queryManager: query.NewManager(queries),
		imageChan:    make(chan scraper.ScrapedImage, 100),
		log:          m.log,
		scraper:      m.scraper,
		ctx:          ctx,
		cancel:       cancel,
		limit:        limit,
	}
	m.jobs[clientName] = job

	job.Start()
	return job.imageChan
}

// Stop stops the scraping job for a client.
func (m *ScrapeManager) Stop(clientName string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if job, exists := m.jobs[clientName]; exists {
		job.Stop()
		delete(m.jobs, clientName)
	}
}

// Start initializes and runs the scraping job.
func (j *ScrapeJob) Start() {
	j.log.Info("Starting new scrape job", "client", j.clientName)
	j.wg.Add(1)
	go j.run()
}

// Stop gracefully stops the scraping job.
func (j *ScrapeJob) Stop() {
	j.log.Info("Stopping scrape job", "client", j.clientName)
	j.cancel()
	j.wg.Wait()
}

// run is the main loop for the scraping job.
func (j *ScrapeJob) run() {
	defer j.wg.Done()
	defer close(j.imageChan)

	sentCount := 0
	for sentCount < j.limit {
		select {
		case <-j.ctx.Done():
			return
		default:
			query, ok := j.queryManager.GetRandom()
			if !ok {
				j.log.Warn("No more queries available, stopping job.", "client", j.clientName)
				return
			}

			j.log.Info("Starting scrape for new random query", "query", query, "client", j.clientName)
			imageChan, err := j.scraper.Scrape(j.ctx, query)
			if err != nil {
				j.log.Error("Failed to start scraping", "query", query, "error", err)
				continue // Try another query
			}

			// Process images from the current query
			for img := range imageChan {
				if sentCount >= j.limit {
					return
				}
				select {
				case j.imageChan <- img:
					sentCount++
				case <-j.ctx.Done():
					return
				}
			}
			j.log.Info("Query exhausted, selecting a new one.", "query", query)
		}
	}
}
