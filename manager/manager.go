package manager

import (
	"context"
	"fmt"
	"gopin/database"
	"gopin/pkg/logger"
	"gopin/query"
	"gopin/scraper"
	"sync"
)

// ScrapeJob represents an active scraping job for a query.
type ScrapeJob struct {
	Query       string
	BaseQuery   string
	Subscribers map[string]chan<- scraper.ScrapedImage
	cancel      func()
}

// ScrapeManager manages the lifecycle of scraping jobs.
type ScrapeManager struct {
	scraper       *scraper.Scraper
	db            *database.DB
	log           *logger.Logger
	jobs          map[string]*ScrapeJob // Maps query to job
	queryManagers map[string]*query.QueryManager
	mu            sync.Mutex
}

// New creates a new ScrapeManager.
func New(scraper *scraper.Scraper, db *database.DB, log *logger.Logger) *ScrapeManager {
	return &ScrapeManager{
		scraper:       scraper,
		db:            db,
		log:           log,
		jobs:          make(map[string]*ScrapeJob),
		queryManagers: make(map[string]*query.QueryManager),
	}
}

// Subscribe allows a client to subscribe to a scraping job for a given list of queries.
func (m *ScrapeManager) Subscribe(clientName string, queries []string) (<-chan scraper.ScrapedImage, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	baseQuery := queries[0] // Use the first query as the base query
	qm := query.NewQueryManager(queries)
	m.queryManagers[baseQuery] = qm

	nextQuery, ok := qm.GetNextQuery()
	if !ok {
		return nil, fmt.Errorf("no queries provided")
	}

	job := &ScrapeJob{
		Query:       nextQuery,
		BaseQuery:   baseQuery,
		Subscribers: make(map[string]chan<- scraper.ScrapedImage),
	}
	ctx, cancel := context.WithCancel(context.Background())
	job.cancel = cancel
	m.jobs[job.Query] = job
	go m.startScrapingJob(ctx, job)

	ch := make(chan scraper.ScrapedImage, 100)
	job.Subscribers[clientName] = ch
	return ch, nil
}

// Unsubscribe removes a client's subscription from a scraping job.
func (m *ScrapeManager) Unsubscribe(clientName, baseQuery string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	var job *ScrapeJob
	var currentQuery string
	for q, j := range m.jobs {
		if j.BaseQuery == baseQuery {
			job = j
			currentQuery = q
			break
		}
	}

	if job == nil {
		return
	}

	if ch, ok := job.Subscribers[clientName]; ok {
		close(ch)
		delete(job.Subscribers, clientName)
	}

	if len(job.Subscribers) == 0 {
		// If there are no more subscribers, cancel the job
		job.cancel()
		delete(m.jobs, currentQuery)
		delete(m.queryManagers, baseQuery)
		m.log.Info("Stopped scraping job due to no subscribers", "query", currentQuery)
	}
}

func (m *ScrapeManager) startScrapingJob(ctx context.Context, job *ScrapeJob) {
	m.log.Info("Starting new scraping job", "query", job.Query, "baseQuery", job.BaseQuery)
	imageChan, err := m.scraper.Scrape(ctx, job.Query)
	if err != nil {
		m.log.Error("Failed to start scraping job", "query", job.Query, "error", err)
		m.handleJobFailure(job)
		return
	}

	for {
		select {
		case <-ctx.Done():
			return
		case image, ok := <-imageChan:
			if !ok {
				m.log.Warn("Scraping channel closed, assuming query is exhausted.", "query", job.Query)
				m.handleJobFailure(job)
				return
			}

			m.mu.Lock()
			currentJob, exists := m.jobs[job.Query]
			if !exists {
				m.mu.Unlock()
				return
			}

			for clientName, subChan := range currentJob.Subscribers {
				select {
				case subChan <- image:
				default:
					m.log.Warn("Subscriber channel is full, dropping image", "client", clientName)
				}
			}
			m.mu.Unlock()
		}
	}
}

func (m *ScrapeManager) handleJobFailure(oldJob *ScrapeJob) {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Ensure the job still exists and has subscribers before trying to chain
	if _, exists := m.jobs[oldJob.Query]; !exists || len(oldJob.Subscribers) == 0 {
		return
	}

	qm, ok := m.queryManagers[oldJob.BaseQuery]
	if !ok {
		m.log.Error("Could not find query manager for base query", "baseQuery", oldJob.BaseQuery)
		return
	}

	newQuery, ok := qm.GetNextQuery()
	if !ok {
		m.log.Warn("All queries exhausted for base query", "baseQuery", oldJob.BaseQuery)
		// Close all subscriber channels
		for _, ch := range oldJob.Subscribers {
			close(ch)
		}
		delete(m.queryManagers, oldJob.BaseQuery)
		return
	}

	m.log.Info("Chaining to new query", "oldQuery", oldJob.Query, "newQuery", newQuery)

	newJob := &ScrapeJob{
		Query:       newQuery,
		BaseQuery:   oldJob.BaseQuery,
		Subscribers: oldJob.Subscribers, // Transfer subscribers
	}
	ctx, cancel := context.WithCancel(context.Background())
	newJob.cancel = cancel
	m.jobs[newQuery] = newJob

	// Remove the old job
	delete(m.jobs, oldJob.Query)

	go m.startScrapingJob(ctx, newJob)
}
