package model

// SubscriptionInfo carries the account metadata an airport reports next to
// its node list (the sublink `subscription-userinfo` / `profile-update-interval`
// response headers). It is captured from every upstream at fetch time and
// re-emitted on deliveries so clients can show expiry and traffic.
type SubscriptionInfo struct {
	// Upload/Download/Total are bytes, as the header defines them.
	Upload   int64 `json:"upload,omitempty"`
	Download int64 `json:"download,omitempty"`
	Total    int64 `json:"total,omitempty"`
	// Expire is unix seconds; 0 means the upstream does not report one.
	Expire int64 `json:"expire,omitempty"`
	// UpdateInterval is profile-update-interval in whole hours; 0 = absent.
	UpdateInterval int `json:"update_interval,omitempty"`
	// Reported distinguishes "upstream sent the header with zero values"
	// from "upstream sent nothing at all".
	Reported bool `json:"reported,omitempty"`
}

// SourceInfo is one upstream's own numbers plus the URL they came from, so
// the console can show per-account expiry/traffic for merged subscriptions.
type SourceInfo struct {
	Source string `json:"source"`
	SubscriptionInfo
}

// MergeSubscriptionInfos folds per-source account data into the single view
// delivered to clients: usage sums across the accounts, expiry takes the
// earliest reported one, refresh interval the shortest. Sources that reported
// nothing are skipped; nil when nobody reported at all.
func MergeSubscriptionInfos(list []*SubscriptionInfo) *SubscriptionInfo {
	out := &SubscriptionInfo{}
	for _, in := range list {
		if in == nil || !in.Reported {
			continue
		}
		out.Reported = true
		out.Upload += in.Upload
		out.Download += in.Download
		out.Total += in.Total
		if in.Expire > 0 && (out.Expire == 0 || in.Expire < out.Expire) {
			out.Expire = in.Expire
		}
		if in.UpdateInterval > 0 && (out.UpdateInterval == 0 || in.UpdateInterval < out.UpdateInterval) {
			out.UpdateInterval = in.UpdateInterval
		}
	}
	if !out.Reported {
		return nil
	}
	return out
}
