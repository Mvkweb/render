package scraper

import (
	"bytes"
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

// Scrape starts the scraping process and returns a channel of scraped images.
func (s *Scraper) Scrape(query string, limit int) (<-chan ScrapedImage, error) {
	results, err := s.client.Scrape(query, limit)
	if err != nil {
		return nil, fmt.Errorf("error scraping pinterest: %w", err)
	}

	imageChan := make(chan pinterest.ScrapeResult, len(results))
	for _, res := range results {
		imageChan <- res
	}
	close(imageChan)

	scrapedImageChan := make(chan ScrapedImage)
	var wg sync.WaitGroup

	wg.Add(s.numWorkers)
	for i := 0; i < s.numWorkers; i++ {
		go func() {
			defer wg.Done()
			for imgResult := range imageChan {
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
				scrapedImageChan <- ScrapedImage{
					Data: imageData,
					Hash: hash,
					ID:   imgResult.ID,
				}
			}
		}()
	}

	// Close the channel once all workers are done
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
