package pinterest

import (
	"context"
	"encoding/json"
	"fmt"
	"gopin/reliability"
	"log/slog"
	"math/rand"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/chromedp/cdproto/cdp"
	"github.com/chromedp/cdproto/network"
	"github.com/chromedp/chromedp"
)

// Client is a client for scraping Pinterest using a headless browser.
type Client struct {
	ctx            context.Context
	log            *slog.Logger
	userAgents     []string
	rateLimiter    *rateLimiter
	circuitBreaker *reliability.CircuitBreaker
}

// NewClient creates a new Pinterest scraper client.
func NewClient(ctx context.Context, log *slog.Logger, userAgents []string) *Client {
	return &Client{
		ctx:            ctx,
		log:            log,
		userAgents:     userAgents,
		rateLimiter:    newRateLimiter(time.Second*5, time.Second*15),
		circuitBreaker: reliability.NewCircuitBreaker(3, time.Minute),
	}
}

// rateLimiter enforces a delay between requests.
type rateLimiter struct {
	lastRequest time.Time
	minDelay    time.Duration
	maxDelay    time.Duration
	mu          sync.Mutex
}

func newRateLimiter(minDelay, maxDelay time.Duration) *rateLimiter {
	return &rateLimiter{
		minDelay: minDelay,
		maxDelay: maxDelay,
	}
}

func (r *rateLimiter) wait() {
	r.mu.Lock()
	defer r.mu.Unlock()

	elapsed := time.Since(r.lastRequest)
	if elapsed < r.minDelay {
		jitter := time.Duration(rand.Intn(1000)) * time.Millisecond
		time.Sleep(r.minDelay - elapsed + jitter)
	}
	r.lastRequest = time.Now()
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
	var results []ScrapeResult
	err := c.circuitBreaker.Call(func() error {
		searchURL := fmt.Sprintf("https://www.pinterest.com/search/pins/?q=%s", url.QueryEscape(query))

		var seenIDs = make(map[string]bool)

		// Channel to receive the response bodies
		responseChan := make(chan []byte)
		listenCtx, cancelListener := context.WithCancel(c.ctx)
		defer cancelListener()

		// Set up a listener for the network responses
		chromedp.ListenTarget(listenCtx, func(ev interface{}) {
			if resp, ok := ev.(*network.EventResponseReceived); ok {
				if strings.Contains(resp.Response.URL, "BaseSearchResource") {
					go func(reqID network.RequestID) {
						body, err := network.GetResponseBody(reqID).Do(cdp.WithExecutor(listenCtx, chromedp.FromContext(listenCtx).Target))
						if err == nil {
							select {
							case responseChan <- body:
							case <-listenCtx.Done():
							}
						}
					}(resp.RequestID)
				}
			}
		})

		// Run the browser tasks
		runErr := chromedp.Run(c.ctx,
			network.Enable(),
			chromedp.Navigate(searchURL),
			chromedp.ActionFunc(func(ctx context.Context) error {
				c.log.Info("Navigated to search page, starting to scroll...", "url", searchURL)

				var noNewResultsCount int
				const maxConsecutiveTimeouts = 3

				// Scroll down to trigger infinite scroll
				for len(results) < limit {
					c.rateLimiter.wait()
					// More human-like scrolling
					err := chromedp.Run(ctx,
						chromedp.Evaluate(`window.scrollBy(0, Math.random() * 500 + 200);`, nil),
						chromedp.Sleep(time.Duration(500+rand.Intn(500))*time.Millisecond),
						chromedp.Evaluate(`window.scrollTo(0, document.body.scrollHeight);`, nil),
					)
					if err != nil {
						return err
					}

					c.log.Info("Scrolled", "results", len(results))

					// Wait for responses or a timeout
					select {
					case body := <-responseChan:
						noNewResultsCount = 0 // Reset counter on new data
						var searchResult SearchResult
						if err := json.Unmarshal(body, &searchResult); err == nil {
							foundNew := false
							for _, pin := range searchResult.ResourceResponse.Data.Results {
								if !seenIDs[pin.ID] && pin.Images.Orig.URL != "" {
									results = append(results, ScrapeResult{ID: pin.ID, URL: pin.Images.Orig.URL})
									seenIDs[pin.ID] = true
									foundNew = true
								}
							}
							if !foundNew {
								c.log.Info("Received data, but no new unique images.")
							}
						}
					case <-time.After(10 * time.Second): // Shorter, repeated timeout
						noNewResultsCount++
						c.log.Warn("Timeout waiting for new results.", "count", noNewResultsCount)
						if noNewResultsCount >= maxConsecutiveTimeouts {
							c.log.Error("Reached max consecutive timeouts, assuming end of page.")
							return nil // Finished
						}
					}
				}
				c.log.Info("Reached image limit", "limit", limit)
				return nil
			}),
		)

		if runErr != nil {
			return fmt.Errorf("failed during browser automation: %w", runErr)
		}
		return nil
	})

	if err != nil {
		return nil, fmt.Errorf("scraping call failed: %w", err)
	}

	return results, nil
}
