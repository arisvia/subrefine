package fetcher_test

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/arisvia/subrefine/internal/core/fetcher"
	"github.com/arisvia/subrefine/internal/core/model"
)

func TestParseSubscriptionInfo(t *testing.T) {
	cases := []struct {
		name    string
		headers map[string]string
		want    *model.SubscriptionInfo
	}{
		{
			name: "full header",
			headers: map[string]string{
				"subscription-userinfo":   "upload=1073741824; download=2147483648; total=10995116277760; expire=1735689600",
				"profile-update-interval": "24",
			},
			want: &model.SubscriptionInfo{Upload: 1 << 30, Download: 2 << 30, Total: 10 * (1 << 40), Expire: 1735689600, UpdateInterval: 24, Reported: true},
		},
		{
			name:    "no spaces after semicolons",
			headers: map[string]string{"subscription-userinfo": "upload=5;download=7;total=0;expire=99"},
			want:    &model.SubscriptionInfo{Upload: 5, Download: 7, Expire: 99, Reported: true},
		},
		{
			name:    "garbage numbers are absent not fatal",
			headers: map[string]string{"subscription-userinfo": "upload=abc; download=1; expire=soon"},
			want:    &model.SubscriptionInfo{Download: 1, Reported: true},
		},
		{
			name:    "only interval",
			headers: map[string]string{"profile-update-interval": "12"},
			want:    &model.SubscriptionInfo{UpdateInterval: 12, Reported: true},
		},
		{
			name:    "no headers",
			headers: nil,
			want:    nil,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			h := http.Header{}
			for k, v := range tc.headers {
				h.Set(k, v)
			}
			got := fetcher.ParseSubscriptionInfo(h)
			if tc.want == nil {
				if got != nil {
					t.Fatalf("expected nil, got %+v", got)
				}
				return
			}
			if got == nil || *got != *tc.want {
				t.Fatalf("got %+v want %+v", got, tc.want)
			}
		})
	}
}

// The fetcher must surface the headers next to the parsed nodes.
func TestFetchReturnsUserInfo(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("subscription-userinfo", "upload=1; download=2; total=3; expire=4")
		w.Write([]byte("ss://YWVzLTEyOC1nY206cA==@1.1.1.1:8388#A"))
	}))
	defer ts.Close()

	nodes, info, err := fetcher.New(5*time.Second).FetchSubscription(t.Context(), ts.URL, "")
	if err != nil {
		t.Fatalf("fetch: %v", err)
	}
	if len(nodes) != 1 {
		t.Fatalf("nodes lost: %d", len(nodes))
	}
	if info == nil || info.Upload != 1 || info.Download != 2 || info.Total != 3 || info.Expire != 4 {
		t.Fatalf("userinfo not captured: %+v", info)
	}
}
