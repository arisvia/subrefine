package auth

import (
	"crypto/subtle"
	"strings"
)

// ValidateAdminToken checks whether the provided authorization header matches expected admin token.
func ValidateAdminToken(authHeader string, expectedToken string) bool {
	if expectedToken == "" {
		return false
	}
	parts := strings.SplitN(authHeader, " ", 2)
	if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") {
		return false
	}
	token := strings.TrimSpace(parts[1])
	return subtle.ConstantTimeCompare([]byte(token), []byte(expectedToken)) == 1
}
