package main

import (
	"flag"
	"fmt"
	"gopin/database"
	"gopin/downloader"
	"gopin/pinterest"
	"log"
)

func main() {
	query := flag.String("query", "art", "The search query.")
	limit := flag.Int("limit", 50, "The maximum number of images to download.")
	outputDir := flag.String("output", "output", "The directory to save images to.")
	numWorkers := flag.Int("workers", 10, "The number of concurrent download workers.")
	dbPath := flag.String("db", "pinterest.db", "The path to the database file.")
	flag.Parse()

	if *query == "" {
		log.Fatalf("Query cannot be empty.")
	}

	db, err := database.Open(*dbPath)
	if err != nil {
		log.Fatalf("Failed to open database: %v", err)
	}
	defer db.Close()

	client, err := pinterest.NewClient()
	if err != nil {
		log.Fatalf("Failed to create client: %v", err)
	}
	defer client.Close()

	fmt.Printf("Starting scrape for query: '%s', downloading to '%s'\n", *query, *outputDir)

	results, err := client.Scrape(*query, *limit)
	if err != nil {
		log.Fatalf("Error scraping pinterest: %v", err)
	}

	imageChan := make(chan pinterest.ScrapeResult, len(results))
	for _, res := range results {
		imageChan <- res
	}
	close(imageChan)

	dl := downloader.NewDownloader(db, *outputDir, *numWorkers)
	dl.Run(imageChan)

	fmt.Println("Done.")
}
