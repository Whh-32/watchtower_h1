package discovery

import (
	"bufio"
	"context"
	"fmt"
	"os/exec"
	"strings"
	"sync"
	"time"
)

type Service struct {
	mu sync.Mutex
}

func NewService() *Service {
	return &Service{}
}

// DiscoverSubdomains uses subfinder to discover subdomains for a given domain
func (s *Service) DiscoverSubdomains(ctx context.Context, domain string) ([]string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Check if subfinder is available
	if _, err := exec.LookPath("subfinder"); err != nil {
		return []string{}, fmt.Errorf("subfinder not found in PATH: %w", err)
	}

	// Use subfinder with timeout (30 seconds per domain)
	cmdCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	// Use subfinder command-line tool with timeout
	cmd := exec.CommandContext(cmdCtx, "subfinder", "-d", domain, "-silent", "-timeout", "20")

	output, err := cmd.Output()
	if err != nil {
		// Check if it's a timeout
		if cmdCtx.Err() == context.DeadlineExceeded {
			return []string{}, fmt.Errorf("subfinder timeout for %s", domain)
		}
		// subfinder might return non-zero exit code but still have results
		// Try to parse output anyway
		if len(output) == 0 {
			return []string{}, fmt.Errorf("subfinder failed: %w", err)
		}
	}

	// Parse output
	scanner := bufio.NewScanner(strings.NewReader(string(output)))
	var subdomains []string

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line != "" {
			subdomains = append(subdomains, line)
		}
	}

	if err := scanner.Err(); err != nil {
		return []string{}, err
	}

	return subdomains, nil
}

// DiscoverDomains discovers domains from a list of base domains
func (s *Service) DiscoverDomains(ctx context.Context, domains []string) ([]string, error) {
	var allSubdomains []string
	var mu sync.Mutex
	var wg sync.WaitGroup

	// Check if subfinder is available first
	if _, err := exec.LookPath("subfinder"); err != nil {
		// If subfinder is not available, return empty (will use base domains only)
		return []string{}, nil
	}

	// Process domains in parallel with timeout
	semaphore := make(chan struct{}, 3) // Limit concurrent subfinder processes to avoid overload

	// Create a timeout context for the entire discovery process (max 5 minutes)
	discoveryCtx, cancel := context.WithTimeout(ctx, 5*time.Minute)
	defer cancel()

	for _, domain := range domains {
		// Check if context is cancelled
		select {
		case <-discoveryCtx.Done():
			break
		default:
		}

		wg.Add(1)
		go func(d string) {
			defer wg.Done()
			semaphore <- struct{}{}
			defer func() { <-semaphore }()

			subdomains, err := s.DiscoverSubdomains(discoveryCtx, d)
			if err != nil {
				// Log error but continue - don't block on failures
				return
			}

			if len(subdomains) > 0 {
				mu.Lock()
				allSubdomains = append(allSubdomains, subdomains...)
				mu.Unlock()
			}
		}(domain)
	}

	// Wait with timeout
	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		// All done
	case <-discoveryCtx.Done():
		// Timeout - return what we have so far
	}

	// Deduplicate
	unique := make(map[string]bool)
	var result []string
	for _, subdomain := range allSubdomains {
		if !unique[subdomain] {
			unique[subdomain] = true
			result = append(result, subdomain)
		}
	}

	return result, nil
}
