package handlers

// Strategy defines the URL shortening strategy.
type Strategy string

const (
	// StrategyToken always generates a new unique code (default).
	StrategyToken Strategy = "token"
	// StrategyHash deduplicates by URL content - same URL returns same code.
	StrategyHash Strategy = "hash"
)

// CreateShortURLRequest is the request body for creating a short URL.
type CreateShortURLRequest struct {
	Body struct {
		URL      string   `doc:"The URL to shorten" example:"https://example.com/very/long/path" json:"url"`
		Strategy Strategy `doc:"Shortening strategy (token or hash)" enum:"token,hash" default:"token" json:"strategy,omitempty"`
	}
}

// CreateShortURLResponse is the response for a successfully created short URL.
type CreateShortURLResponse struct {
	Headers struct {
		Location string `doc:"The short URL location" header:"Location"`
	}
	Body struct {
		Code        string `doc:"The short code"     example:"abc123"                             json:"code"`
		ShortURL    string `doc:"The full short URL" example:"http://localhost:8888/abc123"       json:"shortUrl"`
		OriginalURL string `doc:"The original URL"   example:"https://example.com/very/long/path" json:"originalUrl"`
	}
}

// RedirectRequest is the request for redirecting a short URL.
type RedirectRequest struct {
	Code string `doc:"The short code" example:"abc123" path:"code"`
}

// RedirectResponse is the 301 redirect response.
type RedirectResponse struct {
	Status  int
	Headers struct {
		Location string `doc:"The original URL to redirect to" header:"Location"`
	}
}
