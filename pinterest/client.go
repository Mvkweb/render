package pinterest

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/chromedp/cdproto/cdp"
	"github.com/chromedp/cdproto/network"
	"github.com/chromedp/chromedp"
)

// Client is a client for scraping Pinterest using a headless browser.
type Client struct {
	ctx    context.Context
	cancel context.CancelFunc
}

// NewClient creates a new Pinterest scraper client.
func NewClient() (*Client, error) {
	// Find edge executable path
	var execPath string
	for _, path := range []string{
		`C:\Program Files (x86)\Microsoft\Edge\Application\msedge.exe`,
		`C:\Program Files\Microsoft\Edge\Application\msedge.exe`,
	} {
		if _, err := os.Stat(path); err == nil {
			execPath = path
			break
		}
	}

	if execPath == "" {
		return nil, fmt.Errorf("microsoft edge not found")
	}

	opts := append(chromedp.DefaultExecAllocatorOptions[:],
		chromedp.ExecPath(execPath),
	)

	allocCtx, cancelAlloc := chromedp.NewExecAllocator(context.Background(), opts...)
	ctx, cancel := chromedp.NewContext(allocCtx)
	// Set a timeout to prevent the browser from running indefinitely
	ctx, cancel = context.WithTimeout(ctx, 3*time.Minute)

	client := &Client{
		ctx:    ctx,
		cancel: cancel,
	}

	// It's good practice to ensure the context is cancelled to close the browser
	// We'll wrap the original cancel function.
	originalCancel := client.cancel
	client.cancel = func() {
		originalCancel()
		cancelAlloc()
	}

	return client, nil
}

// Close closes the browser context.
func (c *Client) Close() {
	c.cancel()
}

// ScrapeResult represents an image URL found during scraping.
type ScrapeResult struct {
	ID  string
	URL string
}

// SearchResult represents the structure of the search results from Pinterest's API.
type SearchResult struct {
	ResourceResponse struct {
		Data struct {
			Results []struct {
				ID     string `json:"id"`
				Images struct {
					Orig struct {
						URL string `json:"url"`
					} `json:"orig"`
				} `json:"images"`
			} `json:"results"`
		} `json:"data"`
	} `json:"resource_response"`
}

// Scrape scrapes Pinterest for images based on a keyword.
func (c *Client) Scrape(query string, limit int) ([]ScrapeResult, error) {
	searchURL := fmt.Sprintf("https://www.pinterest.com/search/pins/?q=%s", url.QueryEscape(query))

	var results []ScrapeResult
	var seenIDs = make(map[string]bool)

	// Channel to receive the response bodies
	responseChan := make(chan []byte)
	listenCtx, cancelListener := context.WithCancel(c.ctx)
	defer cancelListener()

	// Set up a listener for the network responses
	chromedp.ListenTarget(listenCtx, func(ev interface{}) {
		if resp, ok := ev.(*network.EventResponseReceived); ok {
			if strings.Contains(resp.Response.URL, "BaseSearchResource") {
				go func() {
					body, err := network.GetResponseBody(resp.RequestID).Do(cdp.WithExecutor(listenCtx, chromedp.FromContext(listenCtx).Target))
					if err == nil {
						select {
						case responseChan <- body:
						case <-listenCtx.Done():
						}
					}
				}()
			}
		}
	})

	// Run the browser tasks
	err := chromedp.Run(c.ctx,
		network.Enable(),
		chromedp.Navigate(searchURL),
		chromedp.ActionFunc(func(ctx context.Context) error {
			log.Println("Navigated to search page, starting to scroll...")
			// Scroll down to trigger infinite scroll
			for len(results) < limit {
				// Execute JavaScript to scroll to the bottom of the page
				err := chromedp.Evaluate(`window.scrollTo(0, document.body.scrollHeight);`, nil).Do(ctx)
				if err != nil {
					return err
				}
				log.Printf("Scrolled, have %d results so far.", len(results))

				// Wait for responses or a timeout
				select {
				case body := <-responseChan:
					var searchResult SearchResult
					if err := json.Unmarshal(body, &searchResult); err == nil {
						for _, pin := range searchResult.ResourceResponse.Data.Results {
							if !seenIDs[pin.ID] && pin.Images.Orig.URL != "" {
								results = append(results, ScrapeResult{ID: pin.ID, URL: pin.Images.Orig.URL})
								seenIDs[pin.ID] = true
							}
						}
					}
				case <-time.After(20 * time.Second):
					log.Println("Timeout waiting for new results, assuming end of page.")
					return nil // Finished
				}
			}
			log.Printf("Reached limit of %d images.", limit)
			return nil
		}),
	)

	if err != nil {
		return nil, fmt.Errorf("failed during browser automation: %w", err)
	}

	return results, nil
}
