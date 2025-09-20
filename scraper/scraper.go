package scraper

import (
	"bytes"
	"context"
	"fmt"
	"gopin/pinterest"
	"gopin/pkg/imaging"
	"gopin/pkg/logger"
	"image"
	_ "image/gif"
	_ "image/jpeg"
	_ "image/png"
	"io"
	"math/rand"
	"net/http"
	"sync"
	"time"

	_ "golang.org/x/image/webp"
)

// ScrapedImage contains the raw data and hash of a scraped image.
type ScrapedImage struct {
	Data []byte
	Hash uint64
	ID   string
}

// Scraper is a service that scrapes images from Pinterest.
type Scraper struct {
	numWorkers int
	log        *logger.Logger
	client     *pinterest.Client
	httpClient *http.Client
	userAgents []string
}

// New creates a new Scraper service.
func New(numWorkers int, log *logger.Logger, userAgents []string) (*Scraper, error) {
	client, err := pinterest.NewClient(log, userAgents)
	if err != nil {
		return nil, fmt.Errorf("failed to create pinterest client: %w", err)
	}

	return &Scraper{
		numWorkers: numWorkers,
		log:        log,
		client:     client,
		httpClient: &http.Client{Timeout: 20 * time.Second},
		userAgents: userAgents,
	}, nil
}

// Scrape starts a continuous scraping process for a given query.
func (s *Scraper) Scrape(ctx context.Context, query string) (<-chan ScrapedImage, error) {
	pinterestImageChan, err := s.client.Scrape(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("error starting pinterest scrape: %w", err)
	}

	scrapedImageChan := make(chan ScrapedImage, s.numWorkers)
	var wg sync.WaitGroup

	// Start worker pool
	wg.Add(s.numWorkers)
	for i := 0; i < s.numWorkers; i++ {
		go func() {
			defer wg.Done()
			for {
				select {
				case <-ctx.Done():
					return
				case imgResult, ok := <-pinterestImageChan:
					if !ok {
						return // Channel closed
					}

					imageData, err := s.downloadImage(imgResult.URL)
					if err != nil {
						s.log.Warn("Failed to download image", "url", imgResult.URL, "error", err)
						continue
					}

					imgDec, _, err := image.Decode(bytes.NewReader(imageData))
					if err != nil {
						s.log.Warn("Failed to decode image", "url", imgResult.URL, "error", err)
						continue
					}

					hash := imaging.DHash(imgDec)
					select {
					case scrapedImageChan <- ScrapedImage{Data: imageData, Hash: hash, ID: imgResult.ID}:
					case <-ctx.Done():
						return
					}
				}
			}
		}()
	}

	// Goroutine to close the channel once all workers are done
	go func() {
		wg.Wait()
		close(scrapedImageChan)
	}()

	return scrapedImageChan, nil
}

func (s *Scraper) downloadImage(url string) ([]byte, error) {
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	// Set a random user agent
	userAgent := s.userAgents[rand.Intn(len(s.userAgents))]
	req.Header.Set("User-Agent", userAgent)
	req.Header.Set("Referer", "https://www.pinterest.com/")

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to download image: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("bad status: %s", resp.Status)
	}

	imageData, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read image body: %w", err)
	}

	return imageData, nil
}

// Close shuts down the scraper's underlying pinterest client.
func (s *Scraper) Close() {
	s.client.Close()
}
