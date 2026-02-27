package scheduler

import (
	"context"
	"fmt"
	"log"
	"strings"
	"sync"
	"time"

	"watchtower/internal/config"
	"watchtower/internal/database"
	"watchtower/internal/discovery"
	"watchtower/internal/hackerone"
	"watchtower/internal/healthcheck"
)

type Scheduler struct {
	db                 *database.DB
	hackeroneClient    *hackerone.Client
	discoveryService   *discovery.Service
	healthCheckService *healthcheck.Service
	config             *config.Config
}

func NewScheduler(
	db *database.DB,
	hackeroneClient *hackerone.Client,
	discoveryService *discovery.Service,
	healthCheckService *healthcheck.Service,
	cfg *config.Config,
) *Scheduler {
	return &Scheduler{
		db:                 db,
		hackeroneClient:    hackeroneClient,
		discoveryService:   discoveryService,
		healthCheckService: healthCheckService,
		config:             cfg,
	}
}

func (s *Scheduler) RunScan() error {
	log.Println("Starting scan...")

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Hour)
	defer cancel()

	// Fetch all programs from HackerOne
	log.Println("Fetching programs from HackerOne...")
	programs, err := s.hackeroneClient.GetAllPrograms()
	if err != nil {
		return fmt.Errorf("failed to fetch programs: %w", err)
	}

	log.Printf("Found %d programs", len(programs))

	// Process programs in parallel (with limit to avoid overwhelming the system)
	semaphore := make(chan struct{}, 5) // Process up to 5 programs concurrently
	var wg sync.WaitGroup

	for _, program := range programs {
		wg.Add(1)
		go func(p hackerone.Program) {
			defer wg.Done()
			semaphore <- struct{}{}
			defer func() { <-semaphore }()

			s.processProgram(ctx, p)
		}(program)
	}

	wg.Wait()

	log.Println("Scan completed successfully")
	return nil
}

func (s *Scheduler) processProgram(ctx context.Context, program hackerone.Program) error {
	log.Printf("Processing program: %s (%s)", program.Attributes.Name, program.Attributes.Handle)

	// Determine program type (RDP/VDP)
	programType := "UNKNOWN"
	submissionState := strings.ToUpper(program.Attributes.SubmissionState)
	if strings.Contains(submissionState, "RDP") || strings.Contains(submissionState, "REMOTE") {
		programType = "RDP"
	} else if strings.Contains(submissionState, "VDP") || strings.Contains(submissionState, "VULNERABILITY") {
		programType = "VDP"
	} else if program.Attributes.OffersBounties {
		programType = "RDP" // If offers bounties, likely RDP
	} else {
		programType = "VDP" // Otherwise likely VDP
	}

	// Save program to database
	dbProgram := &database.Program{
		Name:           program.Attributes.Name,
		Handle:         program.Attributes.Handle,
		URL:            program.Attributes.URL,
		Domain:         program.Attributes.Domain,
		OffersBounties: program.Attributes.OffersBounties,
		ProgramType:    programType,
	}
	if err := s.db.SaveProgram(dbProgram); err != nil {
		log.Printf("Error saving program %s: %v", program.Attributes.Handle, err)
		return err
	}

	// Get program scope
	scopeDomains, err := s.hackeroneClient.GetProgramScope(program.Attributes.Handle)
	if err != nil {
		log.Printf("Error getting scope for %s: %v", program.Attributes.Handle, err)
	}

	// If no scopes found, try to use program domain
	if len(scopeDomains) == 0 {
		if program.Attributes.Domain != "" {
			log.Printf("No structured scopes found for %s, using program domain: %s", program.Attributes.Handle, program.Attributes.Domain)
			scopeDomains = []string{program.Attributes.Domain}
		} else {
			log.Printf("No domains found for program %s (no scopes and no domain attribute)", program.Attributes.Handle)
			return nil // Skip this program but don't error
		}
	} else {
		log.Printf("Found %d scope domains for program %s", len(scopeDomains), program.Attributes.Handle)
	}

		// Discover subdomains (non-blocking - will use base domains if subfinder fails)
		log.Printf("Discovering subdomains for %d base domains in program %s...", len(scopeDomains), program.Attributes.Handle)
		discoveredDomains, err := s.discoveryService.DiscoverDomains(ctx, scopeDomains)
		if err != nil {
			log.Printf("Subdomain discovery failed for %s (will use base domains only): %v", program.Attributes.Handle, err)
			discoveredDomains = []string{} // Use empty, will fall back to base domains
		}

		if len(discoveredDomains) > 0 {
			log.Printf("Discovered %d subdomains for program %s", len(discoveredDomains), program.Attributes.Handle)
		} else {
			log.Printf("No subdomains discovered for %s, using %d base domain(s)", program.Attributes.Handle, len(scopeDomains))
		}

		// Start with base domains, add discovered subdomains
		allDomains := make([]string, len(scopeDomains))
		copy(allDomains, scopeDomains)
		allDomains = append(allDomains, discoveredDomains...)

		// Deduplicate
		uniqueDomains := make(map[string]bool)
		var finalDomains []string
		for _, domain := range allDomains {
			// Clean domain (remove protocol, paths, etc.)
			cleanDomain := cleanDomain(domain)
			if cleanDomain != "" && !uniqueDomains[cleanDomain] {
				uniqueDomains[cleanDomain] = true
				finalDomains = append(finalDomains, cleanDomain)
			}
		}

		// Check health of domains
		log.Printf("Checking health of %d domains for program %s...", len(finalDomains), program.Attributes.Handle)
		healthResults := s.healthCheckService.CheckDomains(ctx, finalDomains)

		// Save domains to database
		for _, result := range healthResults {
			domain := &database.Domain{
				Domain:       result.Domain,
				Program:      program.Attributes.Handle,
				Status:       result.Status,
				DiscoveredAt: time.Now(),
				LastChecked:  time.Now(),
			}
			if err := s.db.SaveDomain(domain); err != nil {
				log.Printf("Error saving domain %s: %v", result.Domain, err)
			}
		}

	log.Printf("Completed processing program %s", program.Attributes.Handle)
	return nil
}

func cleanDomain(domain string) string {
	// Remove protocol
	domain = strings.TrimPrefix(domain, "https://")
	domain = strings.TrimPrefix(domain, "http://")

	// Remove paths
	if idx := strings.Index(domain, "/"); idx != -1 {
		domain = domain[:idx]
	}

	// Remove ports
	if idx := strings.Index(domain, ":"); idx != -1 {
		domain = domain[:idx]
	}

	// Remove wildcards
	domain = strings.TrimPrefix(domain, "*.")

	return strings.TrimSpace(domain)
}
