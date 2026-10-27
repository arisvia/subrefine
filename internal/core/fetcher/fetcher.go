package fetcher

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"time"

	"subrefine/internal/core/model"
	"subrefine/internal/core/parser"
)

type Fetcher struct {
	client *http.Client
}

func New(timeout time.Duration) *Fetcher {
	if timeout <= 0 {
		timeout = 15 * time.Second
	}
	return &Fetcher{
		client: &http.Client{
			Timeout: timeout,
		},
	}
}

// FetchSubscription downloads and parses a subscription URL into ProxyNodes.
func (f *Fetcher) FetchSubscription(ctx context.Context, subURL string, customUA string) ([]*model.ProxyNode, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, subURL, nil)
	if err != nil {
		return nil, fmt.Errorf("create request failed: %w", err)
	}

	if customUA != "" {
		req.Header.Set("User-Agent", customUA)
	} else {
		req.Header.Set("User-Agent", "v2rayN/6.23 subrefine/1.0")
	}

	resp, err := f.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("fetch url failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("unexpected http status: %d", resp.StatusCode)
	}

	// Limit body read to 10MB to avoid OOM
	data, err := io.ReadAll(io.LimitReader(resp.Body, 10*1024*1024))
	if err != nil {
		return nil, fmt.Errorf("read body failed: %w", err)
	}

	return parser.ParseSubscription(data)
}
