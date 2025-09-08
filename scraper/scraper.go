package scraper

import (
	"bytes"
	"fmt"
	"gopin/imaging"
	"gopin/pinterest"
	"image"
	_ "image/jpeg"
	_ "image/png"
	"io"
	"log"
	"net/http"
	"sync"

	_ "golang.org/x/image/webp"
)

// ScrapedImage contains the raw data and hash of a scraped image.
type ScrapedImage struct {
	Data []byte
	Hash uint64
	ID   string
}

// Scraper is a service that scrapes images from Pinterest.
type Scraper struct{}

// New creates a new Scraper service.
func New() *Scraper {
	return &Scraper{}
}

// Scrape starts the scraping process and returns a channel of scraped images.
func (s *Scraper) Scrape(query string, limit int) (<-chan ScrapedImage, error) {
	client, err := pinterest.NewClient()
	if err != nil {
		return nil, fmt.Errorf("failed to create pinterest client: %w", err)
	}
	// The client will be closed by the pinterest package's scrape function.

	results, err := client.Scrape(query, limit)
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
	numWorkers := 10 // TODO: Make this configurable

	wg.Add(numWorkers)
	for i := 0; i < numWorkers; i++ {
		go func() {
			defer wg.Done()
			for imgResult := range imageChan {
				imageData, err := downloadImage(imgResult.URL)
				if err != nil {
					log.Printf("Failed to download image %s: %v", imgResult.URL, err)
					continue
				}

				imgDec, _, err := image.Decode(bytes.NewReader(imageData))
				if err != nil {
					log.Printf("Failed to decode image %s: %v", imgResult.URL, err)
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

func downloadImage(url string) ([]byte, error) {
	resp, err := http.Get(url)
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
