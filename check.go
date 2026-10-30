package main

import (
	"context"
	"net/http"
	"sync"
	"time"
)

// Result is the outcome of checking one link.
type Result struct {
	URL    string `json:"url"`
	Status int    `json:"status,omitempty"`
	Err    string `json:"error,omitempty"`
	Source string `json:"source,omitempty"`
}

// Broken reports whether the link failed or returned a 4xx/5xx status.
// 429 (rate limited) is not counted as broken, since the page likely exists.
func (r Result) Broken() bool {
	if r.Err != "" {
		return true
	}
	return r.Status >= 400 && r.Status != http.StatusTooManyRequests
}

// Checker checks links concurrently.
type Checker struct {
	Client  *http.Client
	Workers int
	Timeout time.Duration
	// Retries is how many extra attempts a link gets after a network error
	// or a 5xx response, which are often temporary.
	Retries int
	// RetryWait is the pause before the first retry; it doubles each time.
	RetryWait time.Duration
}

func NewChecker(workers int, timeout time.Duration) *Checker {
	return &Checker{Client: &http.Client{}, Workers: workers, Timeout: timeout, Retries: 2, RetryWait: 500 * time.Millisecond}
}

// Check checks one URL. It tries HEAD first and falls back to GET, because
// many servers reject HEAD with 403, 404 or 405 even when the page exists.
func (c *Checker) Check(ctx context.Context, url string) Result {
	status, err := c.attempt(ctx, url)
	wait := c.RetryWait
retry:
	for i := 0; i < c.Retries && (err != nil || status >= 500); i++ {
		select {
		case <-ctx.Done():
			break retry
		case <-time.After(wait):
		}
		wait *= 2
		status, err = c.attempt(ctx, url)
	}
	r := Result{URL: url, Status: status}
	if err != nil {
		r.Err = err.Error()
	}
	return r
}

// attempt tries HEAD and falls back to GET.
func (c *Checker) attempt(ctx context.Context, url string) (int, error) {
	status, err := c.request(ctx, http.MethodHead, url)
	if err != nil || status >= 400 {
		status, err = c.request(ctx, http.MethodGet, url)
	}
	return status, err
}

func (c *Checker) request(ctx context.Context, method, url string) (int, error) {
	ctx, cancel := context.WithTimeout(ctx, c.Timeout)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, method, url, nil)
	if err != nil {
		return 0, err
	}
	req.Header.Set("User-Agent", "linkcheck/1.0 (+https://github.com/Ifazawezk/linkcheck)")
	resp, err := c.Client.Do(req)
	if err != nil {
		return 0, err
	}
	resp.Body.Close()
	return resp.StatusCode, nil
}

// CheckAll checks every URL with a pool of workers and returns results in input order.
func (c *Checker) CheckAll(ctx context.Context, urls []string) []Result {
	results := make([]Result, len(urls))
	jobs := make(chan int)
	var wg sync.WaitGroup
	workers := max(1, min(c.Workers, len(urls)))
	for range workers {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := range jobs {
				results[i] = c.Check(ctx, urls[i])
			}
		}()
	}
	for i := range urls {
		jobs <- i
	}
	close(jobs)
	wg.Wait()
	return results
}
