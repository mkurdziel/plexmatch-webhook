package plexmatch

import (
	"fmt"
	"strings"

	"github.com/mkurdziel/plexmatch-webhook/model"
)

// IDSource represents the preferred ID source for matching.
type IDSource string

const (
	IDSourceTvdb IDSource = "tvdb"
	IDSourceTmdb IDSource = "tmdb"
	IDSourceImdb IDSource = "imdb"
)

// SelectBestID returns the provider name and value based on preference order.
// Returns empty strings if no stable ID is available.
func SelectBestID(series *model.Series, prefer IDSource) (provider string, value string) {
	// Build priority order based on preference.
	type candidate struct {
		provider string
		value    string
	}

	var ordered []candidate

	switch prefer {
	case IDSourceTmdb:
		ordered = []candidate{
			{"tmdb", fmtInt(series.TmdbID)},
			{"tvdb", fmtInt(series.TvdbID)},
			{"imdb", series.ImdbID},
		}
	case IDSourceImdb:
		ordered = []candidate{
			{"imdb", series.ImdbID},
			{"tvdb", fmtInt(series.TvdbID)},
			{"tmdb", fmtInt(series.TmdbID)},
		}
	default: // tvdb
		ordered = []candidate{
			{"tvdb", fmtInt(series.TvdbID)},
			{"tmdb", fmtInt(series.TmdbID)},
			{"imdb", series.ImdbID},
		}
	}

	for _, c := range ordered {
		if c.value != "" && c.value != "0" {
			return c.provider, c.value
		}
	}
	return "", ""
}

// Render produces the .plexmatch file content for a given series.
func Render(series *model.Series, prefer IDSource, writeGUID bool) string {
	var b strings.Builder

	b.WriteString("# managed by sonarr-plexmatch\n")

	provider, value := SelectBestID(series, prefer)

	if provider != "" {
		b.WriteString(fmt.Sprintf("%sid: %s\n", provider, value))
		if writeGUID {
			b.WriteString(fmt.Sprintf("guid: %s://%s\n", provider, value))
		}
	}

	if series.Title != "" {
		b.WriteString(fmt.Sprintf("title: %s\n", series.Title))
	}
	if series.Year > 0 {
		b.WriteString(fmt.Sprintf("year: %d\n", series.Year))
	}

	return b.String()
}

func fmtInt(v int) string {
	if v == 0 {
		return ""
	}
	return fmt.Sprintf("%d", v)
}
