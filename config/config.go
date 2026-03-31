package config

import (
	"fmt"
	"os"
	"strings"
)

type Config struct {
	ListenAddr        string
	Port              string
	WebhookSecret     string
	PlexBaseURL       string
	PlexToken         string
	PlexSectionID     string
	PreferIDSource    string
	WriteGUID         bool
	DryRun            bool
	AllowedEventTypes map[string]bool
}

func Load() (*Config, error) {
	c := &Config{
		ListenAddr:     envOrDefault("LISTEN_ADDR", "0.0.0.0"),
		Port:           envOrDefault("PORT", "8080"),
		WebhookSecret:  os.Getenv("WEBHOOK_SECRET"),
		PlexBaseURL:    os.Getenv("PLEX_BASE_URL"),
		PlexToken:      os.Getenv("PLEX_TOKEN"),
		PlexSectionID:  os.Getenv("PLEX_SECTION_ID"),
		PreferIDSource: envOrDefault("PREFER_ID_SOURCE", "tvdb"),
		WriteGUID:      envOrDefault("WRITE_GUID", "true") == "true",
		DryRun:         os.Getenv("DRY_RUN") == "true",
	}

	allowed := envOrDefault("ALLOWED_EVENT_TYPES", "Download,Upgrade,Rename")
	c.AllowedEventTypes = make(map[string]bool)
	for _, et := range strings.Split(allowed, ",") {
		c.AllowedEventTypes[strings.TrimSpace(et)] = true
	}

	if c.PreferIDSource != "tvdb" && c.PreferIDSource != "tmdb" && c.PreferIDSource != "imdb" {
		return nil, fmt.Errorf("invalid PREFER_ID_SOURCE: %s (must be tvdb, tmdb, or imdb)", c.PreferIDSource)
	}

	return c, nil
}

func (c *Config) Addr() string {
	return c.ListenAddr + ":" + c.Port
}

func (c *Config) PlexEnabled() bool {
	return c.PlexBaseURL != "" && c.PlexToken != "" && c.PlexSectionID != ""
}

func envOrDefault(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
