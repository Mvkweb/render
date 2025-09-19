package manager

import (
	"context"
	"fmt"
	"math/rand"
	"os"

	"github.com/chromedp/chromedp"
)

// Manager holds the browser context and cancellation function.
type Manager struct {
	Ctx    context.Context
	Cancel context.CancelFunc
}

// New creates a new browser manager.
func New() (*Manager, error) {
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

	return &Manager{
		Ctx: ctx,
		Cancel: func() {
			cancelCtx()
			cancelAlloc()
		},
	}, nil
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
