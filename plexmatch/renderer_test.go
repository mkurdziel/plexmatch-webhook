package plexmatch

import (
	"testing"

	"github.com/mkurdziel/plexmatch-webhook/model"
)

func TestSelectBestID_PreferTvdb(t *testing.T) {
	s := &model.Series{TvdbID: 123, TmdbID: 456, ImdbID: "tt789"}
	provider, value := SelectBestID(s, IDSourceTvdb)
	if provider != "tvdb" || value != "123" {
		t.Errorf("expected tvdb/123, got %s/%s", provider, value)
	}
}

func TestSelectBestID_PreferTmdb(t *testing.T) {
	s := &model.Series{TvdbID: 123, TmdbID: 456, ImdbID: "tt789"}
	provider, value := SelectBestID(s, IDSourceTmdb)
	if provider != "tmdb" || value != "456" {
		t.Errorf("expected tmdb/456, got %s/%s", provider, value)
	}
}

func TestSelectBestID_PreferImdb(t *testing.T) {
	s := &model.Series{TvdbID: 123, TmdbID: 456, ImdbID: "tt789"}
	provider, value := SelectBestID(s, IDSourceImdb)
	if provider != "imdb" || value != "tt789" {
		t.Errorf("expected imdb/tt789, got %s/%s", provider, value)
	}
}

func TestSelectBestID_FallbackWhenPreferredMissing(t *testing.T) {
	s := &model.Series{TvdbID: 0, TmdbID: 456}
	provider, value := SelectBestID(s, IDSourceTvdb)
	if provider != "tmdb" || value != "456" {
		t.Errorf("expected tmdb/456 fallback, got %s/%s", provider, value)
	}
}

func TestSelectBestID_NoIDs(t *testing.T) {
	s := &model.Series{}
	provider, value := SelectBestID(s, IDSourceTvdb)
	if provider != "" || value != "" {
		t.Errorf("expected empty, got %s/%s", provider, value)
	}
}

func TestRender_Full(t *testing.T) {
	s := &model.Series{
		TvdbID: 76156,
		Title:  "Scrubs",
		Year:   2001,
	}

	content := Render(s, IDSourceTvdb, true)
	expected := "# managed by sonarr-plexmatch\ntvdbid: 76156\nguid: tvdb://76156\ntitle: Scrubs\nyear: 2001\n"
	if content != expected {
		t.Errorf("unexpected content:\ngot:  %q\nwant: %q", content, expected)
	}
}

func TestRender_NoGUID(t *testing.T) {
	s := &model.Series{
		TvdbID: 76156,
		Title:  "Scrubs",
		Year:   2001,
	}

	content := Render(s, IDSourceTvdb, false)
	expected := "# managed by sonarr-plexmatch\ntvdbid: 76156\ntitle: Scrubs\nyear: 2001\n"
	if content != expected {
		t.Errorf("unexpected content:\ngot:  %q\nwant: %q", content, expected)
	}
}

func TestRender_ImdbPreferred(t *testing.T) {
	s := &model.Series{
		ImdbID: "tt0285403",
		Title:  "Scrubs",
		Year:   2001,
	}

	content := Render(s, IDSourceImdb, true)
	expected := "# managed by sonarr-plexmatch\nimdbid: tt0285403\nguid: imdb://tt0285403\ntitle: Scrubs\nyear: 2001\n"
	if content != expected {
		t.Errorf("unexpected content:\ngot:  %q\nwant: %q", content, expected)
	}
}

func TestRender_NoStableID(t *testing.T) {
	s := &model.Series{
		Title: "Unknown Show",
		Year:  2024,
	}

	content := Render(s, IDSourceTvdb, true)
	expected := "# managed by sonarr-plexmatch\ntitle: Unknown Show\nyear: 2024\n"
	if content != expected {
		t.Errorf("unexpected content:\ngot:  %q\nwant: %q", content, expected)
	}
}
