package downloader

import (
	"bytes"
	"fmt"
	"gopin/database"
	"gopin/imaging"
	"gopin/pinterest"
	"image"
	_ "image/jpeg"
	_ "image/png"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"sync"

	_ "golang.org/x/image/webp"
)

// Downloader is a concurrent image downloader.
type Downloader struct {
	db         *database.DB
	outputDir  string
	numWorkers int
}

// NewDownloader creates a new downloader.
func NewDownloader(db *database.DB, outputDir string, numWorkers int) *Downloader {
	return &Downloader{
		db:         db,
		outputDir:  outputDir,
		numWorkers: numWorkers,
	}
}

// Run starts the download process.
func (d *Downloader) Run(images <-chan pinterest.ScrapeResult) {
	var wg sync.WaitGroup
	wg.Add(d.numWorkers)

	for i := 0; i < d.numWorkers; i++ {
		go func() {
			defer wg.Done()
			for img := range images {
				if err := d.processImage(img); err != nil {
					log.Printf("Error processing image %s: %v", img.URL, err)
				}
			}
		}()
	}

	wg.Wait()
}

func (d *Downloader) processImage(img pinterest.ScrapeResult) error {
	// 1. Check if the pin ID is already in the database
	exists, err := d.db.Exists(img.ID)
	if err != nil {
		return fmt.Errorf("failed to check for pin %s: %w", img.ID, err)
	}
	if exists {
		log.Printf("Pin %s already downloaded, skipping.", img.ID)
		return nil
	}

	// 2. Download the image
	resp, err := http.Get(img.URL)
	if err != nil {
		return fmt.Errorf("failed to download image %s: %w", img.URL, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("failed to download image %s: status %s", img.URL, resp.Status)
	}

	// 3. Decode the image and calculate its hash
	imageData, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("failed to read image body: %w", err)
	}

	imgDec, _, err := image.Decode(bytes.NewReader(imageData))
	if err != nil {
		return fmt.Errorf("failed to decode image: %w", err)
	}

	hash := imaging.DHash(imgDec)
	hashStr := fmt.Sprintf("%d", hash)

	// 4. Check if the image hash is already in the database
	exists, err = d.db.Exists(hashStr)
	if err != nil {
		return fmt.Errorf("failed to check for hash %s: %w", hashStr, err)
	}
	if exists {
		log.Printf("Image with hash %s already downloaded, skipping.", hashStr)
		// Also add the pin ID to the database so we don't check it again
		return d.db.Add(img.ID)
	}

	// 5. Save the image
	fileName := filepath.Base(img.URL)
	if err := os.MkdirAll(d.outputDir, 0755); err != nil {
		return fmt.Errorf("failed to create output directory: %w", err)
	}
	filePath := filepath.Join(d.outputDir, fileName)
	if err := os.WriteFile(filePath, imageData, 0644); err != nil {
		return fmt.Errorf("failed to save image: %w", err)
	}
	log.Printf("Saved image %s", filePath)

	// 6. Add the pin ID and hash to the database
	if err := d.db.Add(img.ID); err != nil {
		return fmt.Errorf("failed to add pin %s to db: %w", img.ID, err)
	}
	if err := d.db.Add(hashStr); err != nil {
		return fmt.Errorf("failed to add hash %s to db: %w", hashStr, err)
	}

	return nil
}
