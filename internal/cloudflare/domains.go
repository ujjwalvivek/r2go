package cloudflare

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"time"

	"r2go/internal/model"
)

type Client struct {
	http *http.Client
}

type apiError struct {
	Message string `json:"message"`
}

func checkSuccess(success bool, errors []apiError) error {
	if success {
		return nil
	}
	if len(errors) > 0 && errors[0].Message != "" {
		return fmt.Errorf("Cloudflare API: %s", errors[0].Message)
	}
	return fmt.Errorf("Cloudflare API reported an unsuccessful response")
}

func New() *Client {
	return &Client{http: &http.Client{Timeout: 12 * time.Second}}
}

func (c *Client) PublicDomains(ctx context.Context, accountID, apiToken, bucket string) ([]model.PublicDomain, error) {
	custom, customErr := c.customDomains(ctx, accountID, apiToken, bucket)
	managed, managedErr := c.managedDomain(ctx, accountID, apiToken, bucket)
	if customErr != nil && managedErr != nil {
		return nil, fmt.Errorf("discover public domains: %v; %v", customErr, managedErr)
	}
	out := custom
	if managed.Domain != "" {
		out = append(out, managed)
	}
	return out, nil
}

func (c *Client) customDomains(ctx context.Context, accountID, apiToken, bucket string) ([]model.PublicDomain, error) {
	endpoint := fmt.Sprintf("https://api.cloudflare.com/client/v4/accounts/%s/r2/buckets/%s/domains/custom",
		url.PathEscape(accountID), url.PathEscape(bucket))
	var response struct {
		Success bool `json:"success"`
		Result  struct {
			Domains []struct {
				Domain  string `json:"domain"`
				Enabled bool   `json:"enabled"`
				Status  struct {
					Ownership string `json:"ownership"`
					SSL       string `json:"ssl"`
				} `json:"status"`
			} `json:"domains"`
		} `json:"result"`
		Errors []apiError `json:"errors"`
	}
	if err := c.get(ctx, endpoint, apiToken, &response); err != nil {
		return nil, err
	}
	if err := checkSuccess(response.Success, response.Errors); err != nil {
		return nil, err
	}
	var out []model.PublicDomain
	for _, domain := range response.Result.Domains {
		if domain.Enabled && domain.Status.Ownership == "active" && domain.Status.SSL == "active" {
			out = append(out, model.PublicDomain{Domain: domain.Domain})
		}
	}
	return out, nil
}

func (c *Client) managedDomain(ctx context.Context, accountID, apiToken, bucket string) (model.PublicDomain, error) {
	endpoint := fmt.Sprintf("https://api.cloudflare.com/client/v4/accounts/%s/r2/buckets/%s/domains/managed",
		url.PathEscape(accountID), url.PathEscape(bucket))
	var response struct {
		Success bool `json:"success"`
		Result  struct {
			Domain  string `json:"domain"`
			Enabled bool   `json:"enabled"`
		} `json:"result"`
		Errors []apiError `json:"errors"`
	}
	if err := c.get(ctx, endpoint, apiToken, &response); err != nil {
		return model.PublicDomain{}, err
	}
	if err := checkSuccess(response.Success, response.Errors); err != nil {
		return model.PublicDomain{}, err
	}
	if !response.Result.Enabled {
		return model.PublicDomain{}, nil
	}
	return model.PublicDomain{Domain: response.Result.Domain, Managed: true}, nil
}

func (c *Client) get(ctx context.Context, endpoint, token string, destination any) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Accept", "application/json")
	response, err := c.http.Do(req)
	if err != nil {
		return err
	}
	defer response.Body.Close()
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return fmt.Errorf("Cloudflare API returned %s", response.Status)
	}
	if err := json.NewDecoder(response.Body).Decode(destination); err != nil {
		return fmt.Errorf("decode Cloudflare response: %w", err)
	}
	return nil
}
