package pinterest

import (
	"context"
	"encoding/json"
	"fmt"
	"gopin/pkg/logger"
	"gopin/pkg/reliability"
	"math/rand"
	"net/url"
	"os"
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
	log            *logger.Logger
	userAgents     []string
	rateLimiter    *rateLimiter
	circuitBreaker *reliability.CircuitBreaker
	cancel         context.CancelFunc
}

// NewClient creates a new Pinterest scraper client.
func NewClient(log *logger.Logger, userAgents []string) (*Client, error) {
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
		chromedp.UserAgent(getRandomUserAgent()),
		chromedp.WindowSize(1920+rand.Intn(200), 1080+rand.Intn(200)),
		chromedp.DisableGPU,
		chromedp.Flag("disable-blink-features", "AutomationControlled"),
		chromedp.Flag("excludeSwitches", "enable-automation"),
	)

	allocCtx, cancelAlloc := chromedp.NewExecAllocator(context.Background(), opts...)
	ctx, cancelCtx := chromedp.NewContext(allocCtx)

	client := &Client{
		ctx:            ctx,
		log:            log,
		userAgents:     userAgents,
		rateLimiter:    newRateLimiter(time.Second*5, time.Second*15),
		circuitBreaker: reliability.NewCircuitBreaker(3, time.Minute),
		cancel: func() {
			cancelCtx()
			cancelAlloc()
		},
	}

	return client, nil
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

var ErrQueryExhausted = fmt.Errorf("query exhausted")

// Scrape starts a continuous scraping process for a given query.
func (c *Client) Scrape(ctx context.Context, query string) (<-chan ScrapeResult, error) {
	resultChan := make(chan ScrapeResult, 100)

	go func() {
		defer close(resultChan)

		err := c.circuitBreaker.Call(func() error {
			return c.scrapeWithRetries(ctx, query, resultChan)
		})

		if err != nil && err != ErrQueryExhausted {
			c.log.Error("Scraping call failed after multiple retries.", "error", err, "query", query)
		}
	}()

	return resultChan, nil
}

func (c *Client) scrapeWithRetries(ctx context.Context, query string, resultChan chan<- ScrapeResult) error {
	searchURL := fmt.Sprintf("https://www.pinterest.com/search/pins/?q=%s", url.QueryEscape(query))
	var seenIDs = make(map[string]bool)
	responseChan := make(chan []byte)

	listenCtx, cancelListener := context.WithCancel(c.ctx)
	defer cancelListener()

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

	return chromedp.Run(c.ctx,
		network.Enable(),
		chromedp.Navigate(searchURL),
		chromedp.ActionFunc(func(actCtx context.Context) error {
			c.log.Info("Navigated to search page, starting to scroll...", "url", searchURL)
			var noNewResultsCount int
			const maxConsecutiveTimeouts = 5

			for {
				select {
				case <-ctx.Done():
					c.log.Info("Scraping cancelled by parent context.", "query", query)
					return nil
				default:
					c.rateLimiter.wait()
					err := chromedp.Run(actCtx,
						chromedp.Evaluate(`window.scrollBy(0, Math.random() * 800 + 200);`, nil),
						chromedp.Sleep(time.Duration(700+rand.Intn(800))*time.Millisecond),
						chromedp.Evaluate(`window.scrollTo(0, document.body.scrollHeight);`, nil),
					)
					if err != nil {
						return err
					}

					select {
					case body := <-responseChan:
						noNewResultsCount = 0
						var searchResult SearchResult
						if err := json.Unmarshal(body, &searchResult); err == nil {
							if len(searchResult.ResourceResponse.Data.Results) == 0 {
								c.log.Info("Received response with no image results.", "query", query)
								noNewResultsCount++
							}

							for _, pin := range searchResult.ResourceResponse.Data.Results {
								if !seenIDs[pin.ID] && pin.Images.Orig.URL != "" {
									seenIDs[pin.ID] = true
									select {
									case resultChan <- ScrapeResult{ID: pin.ID, URL: pin.Images.Orig.URL}:
									case <-ctx.Done():
										return nil
									}
								}
							}
						}
					case <-time.After(20 * time.Second): // Increased timeout
						noNewResultsCount++
						c.log.Warn("Timeout waiting for new results.", "count", noNewResultsCount)
					}

					if noNewResultsCount >= maxConsecutiveTimeouts {
						c.log.Warn("Reached max consecutive timeouts, assuming query is exhausted.", "query", query)
						return ErrQueryExhausted
					}
				}
			}
		}),
	)
}

func (c *Client) Close() {
	c.cancel()
}

func getRandomUserAgent() string {
	agents := []string{
		"Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/108.0.0.0 Safari/537.36",
		"Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/107.0.0.0 Safari/536.36",
		"Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/108.0.0.0 Safari/537.36",
		"Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/107.0.0.0 Safari/537.36",
	}
	return agents[rand.Intn(len(agents))]
}
