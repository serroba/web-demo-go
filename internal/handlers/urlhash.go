package handlers

import (
	"crypto/sha256"
	"encoding/hex"
	"net/url"
	"strings"
)

// NormalizeURL normalizes a URL for consistent hashing.
// - Lowercases the scheme and host
// - Removes default ports (80 for http, 443 for https)
// - Removes trailing slashes from path (unless path is just "/")
// - Removes empty fragment
func NormalizeURL(rawURL string) (string, error) {
	u, err := url.Parse(rawURL)
	if err != nil {
		return "", err
	}

	// Lowercase scheme and host
	u.Scheme = strings.ToLower(u.Scheme)
	u.Host = strings.ToLower(u.Host)

	// Remove default ports
	host := u.Host
	if strings.HasSuffix(host, ":80") && u.Scheme == "http" {
		u.Host = strings.TrimSuffix(host, ":80")
	} else if strings.HasSuffix(host, ":443") && u.Scheme == "https" {
		u.Host = strings.TrimSuffix(host, ":443")
	}

	// Remove trailing slash from path (but keep "/" for root)
	if len(u.Path) > 1 && strings.HasSuffix(u.Path, "/") {
		u.Path = strings.TrimSuffix(u.Path, "/")
	}

	// Remove empty fragment
	u.Fragment = ""

	return u.String(), nil
}

// HashURL computes a SHA256 hash of the normalized URL.
// Returns the hash as a hex-encoded string.
func HashURL(normalizedURL string) string {
	h := sha256.Sum256([]byte(normalizedURL))
	return hex.EncodeToString(h[:])
}
