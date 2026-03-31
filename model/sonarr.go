package model

// SonarrWebhook represents the incoming webhook payload from Sonarr.
type SonarrWebhook struct {
	EventType string  `json:"eventType"`
	Series    *Series `json:"series"`
}

// Series contains the series metadata from the Sonarr payload.
type Series struct {
	Path   string `json:"path"`
	Title  string `json:"title"`
	Year   int    `json:"year"`
	TvdbID int    `json:"tvdbId"`
	TmdbID int    `json:"tmdbId,omitempty"`
	ImdbID string `json:"imdbId,omitempty"`
}

// HasStableID returns true if at least one provider ID is present.
func (s *Series) HasStableID() bool {
	return s.TvdbID > 0 || s.TmdbID > 0 || s.ImdbID != ""
}
