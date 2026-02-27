package enrichment

import (
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"sync"
	"time"
)

type Service struct{}

func NewService() *Service {
	return &Service{}
}

type DomainDetails struct {
	Domain       string
	Status       string
	StatusCode   int
	Title        string
	Technologies []string
	Server       string
	ContentType  string
	ContentLength int64
}

// EnrichDomain uses httpx to get detailed information about a domain
func (s *Service) EnrichDomain(ctx context.Context, domain string) (*DomainDetails, error) {
	// Check if httpx is available
	if _, err := exec.LookPath("httpx"); err != nil {
		return nil, fmt.Errorf("httpx not found in PATH: %w", err)
	}

	// Create context with timeout
	cmdCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	// Run httpx with JSON output
	cmd := exec.CommandContext(cmdCtx, "httpx", 
		"-u", fmt.Sprintf("https://%s", domain),
		"-json",
		"-title",
		"-tech-detect",
		"-status-code",
		"-silent",
		"-timeout", "10",
	)

	output, err := cmd.Output()
	if err != nil {
		// Try HTTP if HTTPS fails
		return s.enrichDomainHTTP(ctx, domain)
	}

	if len(output) == 0 {
		return s.enrichDomainHTTP(ctx, domain)
	}

	// Parse JSON output
	var httpxResult struct {
		URL           string   `json:"url"`
		StatusCode    int      `json:"status_code"`
		Title         string   `json:"title"`
		Technologies  []string `json:"technologies"`
		Server        string   `json:"server"`
		ContentType   string   `json:"content_type"`
		ContentLength int64    `json:"content_length"`
	}

	if err := json.Unmarshal(output, &httpxResult); err != nil {
		// If JSON parsing fails, try HTTP
		return s.enrichDomainHTTP(ctx, domain)
	}

	return &DomainDetails{
		Domain:        domain,
		Status:        "up",
		StatusCode:    httpxResult.StatusCode,
		Title:         httpxResult.Title,
		Technologies:  httpxResult.Technologies,
		Server:        httpxResult.Server,
		ContentType:   httpxResult.ContentType,
		ContentLength: httpxResult.ContentLength,
	}, nil
}

func (s *Service) enrichDomainHTTP(ctx context.Context, domain string) (*DomainDetails, error) {
	cmdCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	cmd := exec.CommandContext(cmdCtx, "httpx",
		"-u", fmt.Sprintf("http://%s", domain),
		"-json",
		"-title",
		"-tech-detect",
		"-status-code",
		"-silent",
		"-timeout", "10",
	)

	output, err := cmd.Output()
	if err != nil {
		return &DomainDetails{
			Domain: domain,
			Status: "down",
		}, nil
	}

	if len(output) == 0 {
		return &DomainDetails{
			Domain: domain,
			Status: "down",
		}, nil
	}

	var httpxResult struct {
		URL           string   `json:"url"`
		StatusCode    int      `json:"status_code"`
		Title         string   `json:"title"`
		Technologies  []string `json:"technologies"`
		Server        string   `json:"server"`
		ContentType   string   `json:"content_type"`
		ContentLength int64    `json:"content_length"`
	}

	if err := json.Unmarshal(output, &httpxResult); err != nil {
		return &DomainDetails{
			Domain: domain,
			Status: "unknown",
		}, nil
	}

	return &DomainDetails{
		Domain:        domain,
		Status:        "up",
		StatusCode:    httpxResult.StatusCode,
		Title:         httpxResult.Title,
		Technologies:  httpxResult.Technologies,
		Server:        httpxResult.Server,
		ContentType:   httpxResult.ContentType,
		ContentLength: httpxResult.ContentLength,
	}, nil
}

// EnrichDomains enriches multiple domains in parallel
func (s *Service) EnrichDomains(ctx context.Context, domains []string) map[string]*DomainDetails {
	results := make(map[string]*DomainDetails)
	semaphore := make(chan struct{}, 10) // Limit concurrent httpx processes
	var mu sync.Mutex
	var wg sync.WaitGroup

	for _, domain := range domains {
		wg.Add(1)
		go func(d string) {
			defer wg.Done()
			semaphore <- struct{}{}
			defer func() { <-semaphore }()

			details, err := s.EnrichDomain(ctx, d)
			if err == nil && details != nil {
				mu.Lock()
				results[d] = details
				mu.Unlock()
			}
		}(domain)
	}

	wg.Wait()
	return results
}
