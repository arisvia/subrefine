package fetcher

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/arisvia/subrefine/internal/core/model"
	"github.com/arisvia/subrefine/internal/core/parser"
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

// FetchSubscription downloads and parses a subscription URL into ProxyNodes,
// returning the airport-reported account metadata (expiry/traffic headers)
// alongside - nil when the upstream sends none.
func (f *Fetcher) FetchSubscription(ctx context.Context, subURL string, customUA string) ([]*model.ProxyNode, *model.SubscriptionInfo, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, subURL, nil)
	if err != nil {
		return nil, nil, fmt.Errorf("create request failed: %w", err)
	}

	if customUA != "" {
		req.Header.Set("User-Agent", customUA)
	} else {
		req.Header.Set("User-Agent", "v2rayN/6.23 subrefine/1.0")
	}

	resp, err := f.client.Do(req)
	if err != nil {
		return nil, nil, fmt.Errorf("fetch url failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, nil, fmt.Errorf("unexpected http status: %d", resp.StatusCode)
	}

	// Limit body read to 10MB to avoid OOM
	data, err := io.ReadAll(io.LimitReader(resp.Body, 10*1024*1024))
	if err != nil {
		return nil, nil, fmt.Errorf("read body failed: %w", err)
	}

	nodes, err := parser.ParseSubscription(data)
	if err != nil {
		return nil, nil, err
	}
	return nodes, ParseSubscriptionInfo(resp.Header), nil
}

// ParseSubscriptionInfo extracts the sublink account headers. Values are
// `key=value` pairs separated by semicolons, with or without spacing;
// unparsable numbers are treated as absent rather than failing the fetch -
// the node list is the payload, this is decoration.
func ParseSubscriptionInfo(h http.Header) *model.SubscriptionInfo {
	info := &model.SubscriptionInfo{}
	if raw := h.Get("subscription-userinfo"); raw != "" {
		for _, pair := range strings.Split(raw, ";") {
			k, v, ok := strings.Cut(strings.TrimSpace(pair), "=")
			if !ok {
				continue
			}
			n, err := strconv.ParseInt(strings.TrimSpace(v), 10, 64)
			if err != nil {
				continue
			}
			switch strings.ToLower(strings.TrimSpace(k)) {
			case "upload":
				info.Upload = n
			case "download":
				info.Download = n
			case "total":
				info.Total = n
			case "expire":
				info.Expire = n
			}
		}
		info.Reported = true
	}
	if raw := h.Get("profile-update-interval"); raw != "" {
		if n, err := strconv.Atoi(strings.TrimSpace(raw)); err == nil && n > 0 {
			info.UpdateInterval = n
			info.Reported = true
		}
	}
	if !info.Reported {
		return nil
	}
	return info
}
