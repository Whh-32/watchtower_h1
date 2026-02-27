package hackerone

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

type Client struct {
	token      string
	httpClient *http.Client
	baseURL    string
}

type Program struct {
	ID         string `json:"id"`
	Type       string `json:"type"`
	Attributes struct {
		Name            string `json:"name"`
		Handle          string `json:"handle"`
		URL             string `json:"url"`
		Domain          string `json:"domain"`
		OffersBounties  bool   `json:"offers_bounties"`
		SubmissionState string `json:"submission_state"`
	} `json:"attributes"`
}

type ProgramsResponse struct {
	Data  []Program `json:"data"`
	Links struct {
		Next *string `json:"next"`
	} `json:"links"`
}

func NewClient(token string) *Client {
	// Trim whitespace from token
	token = strings.TrimSpace(token)
	return &Client{
		token: token,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
		baseURL: "https://api.hackerone.com/v1",
	}
}

// setAuth sets the appropriate authentication header
// HackerOne API supports both Basic Auth (username:token) and Bearer token
func (c *Client) setAuth(req *http.Request) {
	if c.token == "" {
		return
	}

	// Check if token contains a colon (username:token format)
	if strings.Contains(c.token, ":") {
		// Split into username and token
		parts := strings.SplitN(c.token, ":", 2)
		if len(parts) == 2 {
			req.SetBasicAuth(parts[0], parts[1])
		} else {
			// Fallback: use token as Bearer
			req.Header.Set("Authorization", "Bearer "+c.token)
		}
	} else {
		// Try Bearer token first (most common for HackerOne)
		req.Header.Set("Authorization", "Bearer "+c.token)
	}
}

func (c *Client) GetAllPrograms() ([]Program, error) {
	var allPrograms []Program
	url := fmt.Sprintf("%s/hackers/programs", c.baseURL)

	for url != "" {
		req, err := http.NewRequest("GET", url, nil)
		if err != nil {
			return nil, err
		}

		c.setAuth(req)
		req.Header.Set("Accept", "application/json")

		resp, err := c.httpClient.Do(req)
		if err != nil {
			return nil, err
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			body, _ := io.ReadAll(resp.Body)
			if resp.StatusCode == http.StatusUnauthorized {
				return nil, fmt.Errorf("HackerOne API authentication failed (401). Please check your API token. Token format should be either 'username:token' for Basic Auth or just the token for Bearer Auth. Error: %s", string(body))
			}
			return nil, fmt.Errorf("HackerOne API error: %d - %s", resp.StatusCode, string(body))
		}

		var programsResp ProgramsResponse
		if err := json.NewDecoder(resp.Body).Decode(&programsResp); err != nil {
			return nil, err
		}

		allPrograms = append(allPrograms, programsResp.Data...)

		// Check for next page
		if programsResp.Links.Next != nil {
			url = *programsResp.Links.Next
		} else {
			url = ""
		}

		// Rate limiting - be respectful
		time.Sleep(500 * time.Millisecond)
	}

	return allPrograms, nil
}

func (c *Client) GetProgramScope(handle string) ([]string, error) {
	// Try the direct structured_scopes endpoint first (more reliable)
	domains, err := c.getProgramScopesDirect(handle)
	if err == nil && len(domains) > 0 {
		return domains, nil
	}

	// Fallback: try to get from program endpoint with included scopes
	url := fmt.Sprintf("%s/hackers/programs/%s?include=structured_scopes", c.baseURL, handle)

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}

	c.setAuth(req)
	req.Header.Set("Accept", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		if resp.StatusCode == http.StatusUnauthorized {
			return nil, fmt.Errorf("HackerOne API authentication failed (401) for program scope. Please check your API token. Error: %s", string(body))
		}
		// If we can't get scopes, return empty (will fall back to program domain)
		return []string{}, nil
	}

	// Parse JSON:API format with included data
	var programResponse struct {
		Data struct {
			ID         string `json:"id"`
			Type       string `json:"type"`
			Attributes struct {
				Name   string `json:"name"`
				Handle string `json:"handle"`
			} `json:"attributes"`
			Relationships struct {
				StructuredScopes struct {
					Data []struct {
						ID   string `json:"id"`
						Type string `json:"type"`
					} `json:"data"`
				} `json:"structured_scopes"`
			} `json:"relationships"`
		} `json:"data"`
		Included []struct {
			ID         string `json:"id"`
			Type       string `json:"type"`
			Attributes struct {
				AssetIdentifier       string `json:"asset_identifier"`
				AssetType             string `json:"asset_type"`
				EligibleForBounty     bool   `json:"eligible_for_bounty"`
				EligibleForSubmission bool   `json:"eligible_for_submission"`
			} `json:"attributes"`
		} `json:"included"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&programResponse); err != nil {
		// If parsing fails, return empty (will use program domain as fallback)
		return []string{}, nil
	}

	// Map scope IDs to actual scope data from included array
	scopeMap := make(map[string]struct {
		AssetIdentifier string
		AssetType       string
	})
	for _, included := range programResponse.Included {
		if included.Type == "structured-scope" {
			scopeMap[included.ID] = struct {
				AssetIdentifier string
				AssetType       string
			}{
				AssetIdentifier: included.Attributes.AssetIdentifier,
				AssetType:       included.Attributes.AssetType,
			}
		}
	}

	var resultDomains []string
	for _, scopeRef := range programResponse.Data.Relationships.StructuredScopes.Data {
		if scope, ok := scopeMap[scopeRef.ID]; ok {
			// Include domains, URLs, and wildcards
			if scope.AssetType == "URL" || scope.AssetType == "DOMAIN" || scope.AssetType == "WILDCARD" {
				resultDomains = append(resultDomains, scope.AssetIdentifier)
			}
		}
	}

	return resultDomains, nil
}

// getProgramScopesDirect tries to get scopes using the direct structured_scopes endpoint
func (c *Client) getProgramScopesDirect(handle string) ([]string, error) {
	url := fmt.Sprintf("%s/hackers/programs/%s/structured_scopes", c.baseURL, handle)

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}

	c.setAuth(req)
	req.Header.Set("Accept", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		// If this endpoint doesn't work, return empty (will fall back to program domain)
		return []string{}, nil
	}

	var scopesResponse struct {
		Data []struct {
			Attributes struct {
				AssetIdentifier       string `json:"asset_identifier"`
				AssetType             string `json:"asset_type"`
				EligibleForBounty     bool   `json:"eligible_for_bounty"`
				EligibleForSubmission bool   `json:"eligible_for_submission"`
			} `json:"attributes"`
		} `json:"data"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&scopesResponse); err != nil {
		return []string{}, nil
	}

	var domains []string
	for _, scope := range scopesResponse.Data {
		// Include all asset types that could be domains
		assetType := scope.Attributes.AssetType
		if assetType == "URL" || assetType == "DOMAIN" || assetType == "WILDCARD" {
			domains = append(domains, scope.Attributes.AssetIdentifier)
		}
	}

	return domains, nil
}
