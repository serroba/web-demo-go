package handlers

import (
	"github.com/danielgtaylor/huma/v2"
)

func RegisterRoutes(api huma.API, urlHandler *URLHandler) {
	huma.Post(api, "/shorten", urlHandler.CreateShortURL)
	huma.Get(api, "/{code}", urlHandler.RedirectToURL)
}
