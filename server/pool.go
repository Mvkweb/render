package server

import (
	"fmt"
	"gopin/database"
	"gopin/scraper"
	"math/rand"
	"sync"
	"time"
)

// ImagePool holds a collection of scraped images to be served to clients.
type ImagePool struct {
	images      []scraper.ScrapedImage
	mu          sync.RWMutex
	maxSize     int
	lastRefresh time.Time
}

// NewImagePool creates a new ImagePool.
func NewImagePool(maxSize int) *ImagePool {
	return &ImagePool{
		images:  make([]scraper.ScrapedImage, 0),
		maxSize: maxSize,
	}
}

// AddImages adds a slice of images to the pool.
func (ip *ImagePool) AddImages(images []scraper.ScrapedImage) {
	ip.mu.Lock()
	defer ip.mu.Unlock()

	// Add new images, remove old ones if over limit
	ip.images = append(ip.images, images...)
	if len(ip.images) > ip.maxSize {
		// Keep only the newest images
		ip.images = ip.images[len(ip.images)-ip.maxSize:]
	}
	ip.lastRefresh = time.Now()
}

// GetRandomUnseenImage gets a random image from the pool that the client has not seen.
func (ip *ImagePool) GetRandomUnseenImage(db *database.DB, clientName string) (*scraper.ScrapedImage, error) {
	ip.mu.RLock()
	defer ip.mu.RUnlock()

	// Shuffle and find an unseen image
	indices := rand.Perm(len(ip.images))
	for _, i := range indices {
		img := &ip.images[i]
		seen, err := db.HasClientSeenImage(clientName, img.Hash)
		if err != nil {
			continue
		}
		if !seen {
			return img, nil
		}
	}
	return nil, fmt.Errorf("no unseen images in pool")
}
