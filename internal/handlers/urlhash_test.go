package handlers

import "testing"

func TestNormalizeURL(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "lowercase host",
			input:    "HTTPS://EXAMPLE.COM/path",
			expected: "https://example.com/path",
		},
		{
			name:     "lowercase scheme",
			input:    "HTTPS://example.com/path",
			expected: "https://example.com/path",
		},
		{
			name:     "remove trailing slash",
			input:    "https://example.com/path/",
			expected: "https://example.com/path",
		},
		{
			name:     "keep root slash",
			input:    "https://example.com/",
			expected: "https://example.com/",
		},
		{
			name:     "remove default https port",
			input:    "https://example.com:443/path",
			expected: "https://example.com/path",
		},
		{
			name:     "remove default http port",
			input:    "http://example.com:80/path",
			expected: "http://example.com/path",
		},
		{
			name:     "keep non-default port",
			input:    "https://example.com:8080/path",
			expected: "https://example.com:8080/path",
		},
		{
			name:     "remove fragment",
			input:    "https://example.com/path#section",
			expected: "https://example.com/path",
		},
		{
			name:     "preserve query string",
			input:    "https://example.com/path?foo=bar",
			expected: "https://example.com/path?foo=bar",
		},
		{
			name:     "complex url normalization",
			input:    "HTTPS://EXAMPLE.COM:443/path/?foo=bar#section",
			expected: "https://example.com/path?foo=bar",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := NormalizeURL(tt.input)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if result != tt.expected {
				t.Errorf("got %q, want %q", result, tt.expected)
			}
		})
	}
}

func TestNormalizeURL_InvalidURL(t *testing.T) {
	_, err := NormalizeURL("://invalid")
	if err == nil {
		t.Error("expected error for invalid URL, got nil")
	}
}

func TestHashURL(t *testing.T) {
	t.Run("same input produces same hash", func(t *testing.T) {
		hash1 := HashURL("https://example.com/path")
		hash2 := HashURL("https://example.com/path")
		if hash1 != hash2 {
			t.Errorf("same input produced different hashes: %q vs %q", hash1, hash2)
		}
	})

	t.Run("different input produces different hash", func(t *testing.T) {
		hash1 := HashURL("https://example.com/path1")
		hash2 := HashURL("https://example.com/path2")
		if hash1 == hash2 {
			t.Error("different inputs produced same hash")
		}
	})

	t.Run("hash is 64 hex characters (SHA256)", func(t *testing.T) {
		hash := HashURL("https://example.com/path")
		if len(hash) != 64 {
			t.Errorf("hash length is %d, expected 64", len(hash))
		}
	})

	t.Run("hash contains only hex characters", func(t *testing.T) {
		hash := HashURL("https://example.com/path")
		for _, c := range hash {
			if !((c >= '0' && c <= '9') || (c >= 'a' && c <= 'f')) {
				t.Errorf("hash contains non-hex character: %c", c)
			}
		}
	})
}

func TestNormalizeAndHash_Equivalence(t *testing.T) {
	// URLs that should normalize to the same value should produce the same hash
	equivalentURLs := []string{
		"https://example.com/path",
		"HTTPS://EXAMPLE.COM/path",
		"https://example.com:443/path",
		"https://EXAMPLE.COM:443/path/",
	}

	var firstHash string
	for i, url := range equivalentURLs {
		normalized, err := NormalizeURL(url)
		if err != nil {
			t.Fatalf("failed to normalize %q: %v", url, err)
		}
		hash := HashURL(normalized)

		if i == 0 {
			firstHash = hash
		} else if hash != firstHash {
			t.Errorf("URL %q produced different hash than first URL\ngot:  %s\nwant: %s", url, hash, firstHash)
		}
	}
}
