package healthcheck

import (
	"context"
	"fmt"
	"net/http"
	"sync"
	"time"
)

type Service struct {
	timeout time.Duration
	workers int
	client  *http.Client
}

func NewService(timeout time.Duration, workers int) *Service {
	return &Service{
		timeout: timeout,
		workers: workers,
		client: &http.Client{
			Timeout: timeout,
			Transport: &http.Transport{
				MaxIdleConns:        100,
				MaxIdleConnsPerHost: 10,
				IdleConnTimeout:     30 * time.Second,
			},
		},
	}
}

type CheckResult struct {
	Domain string
	Status string // "up", "down", "unknown"
	Error  error
}

func (s *Service) CheckDomain(ctx context.Context, domain string) CheckResult {
	// Try HTTPS first, then HTTP
	urls := []string{
		fmt.Sprintf("https://%s", domain),
		fmt.Sprintf("http://%s", domain),
	}

	for _, url := range urls {
		req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
		if err != nil {
			continue
		}

		req.Header.Set("User-Agent", "Watchtower/1.0")

		resp, err := s.client.Do(req)
		if err != nil {
			continue
		}
		resp.Body.Close()

		// Consider 2xx, 3xx, and even 4xx as "up" (server is responding)
		if resp.StatusCode < 500 {
			return CheckResult{
				Domain: domain,
				Status: "up",
			}
		}
	}

	return CheckResult{
		Domain: domain,
		Status: "down",
		Error:  fmt.Errorf("domain not reachable"),
	}
}

func (s *Service) CheckDomains(ctx context.Context, domains []string) []CheckResult {
	results := make([]CheckResult, len(domains))

	// Create worker pool
	domainChan := make(chan string, len(domains))
	resultChan := make(chan CheckResult, len(domains))

	// Send domains to channel
	for _, domain := range domains {
		domainChan <- domain
	}
	close(domainChan)

	// Start workers
	var wg sync.WaitGroup
	for i := 0; i < s.workers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for domain := range domainChan {
				select {
				case <-ctx.Done():
					return
				default:
					result := s.CheckDomain(ctx, domain)
					resultChan <- result
				}
			}
		}()
	}

	// Wait for all workers to finish
	go func() {
		wg.Wait()
		close(resultChan)
	}()

	// Collect results
	index := 0
	resultMap := make(map[string]CheckResult)
	for result := range resultChan {
		resultMap[result.Domain] = result
	}

	// Preserve order
	for _, domain := range domains {
		if result, ok := resultMap[domain]; ok {
			results[index] = result
		} else {
			results[index] = CheckResult{
				Domain: domain,
				Status: "unknown",
			}
		}
		index++
	}

	return results
}
