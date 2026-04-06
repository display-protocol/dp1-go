package playlist

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/display-protocol/dp1-go/extension/playlists"
	"github.com/display-protocol/dp1-go/internal/validate"
)

func TestHydrateDynamicQueryString(t *testing.T) {
	t.Parallel()
	got, err := HydrateDynamicQueryString(`owner={{a}}&x={{b}}`, HydrationParams{"a": "1", "b": "2"})
	if err != nil {
		t.Fatal(err)
	}
	if got != `owner=1&x=2` {
		t.Fatal(got)
	}
	empty, err := HydrateDynamicQueryString("", HydrationParams{"a": "1"})
	if err != nil || empty != "" {
		t.Fatalf("%q %v", empty, err)
	}
	_, err = HydrateDynamicQueryString(`{{missing}}`, HydrationParams{})
	if !errors.Is(err, ErrDynamicQueryHydration) {
		t.Fatalf("got %v", err)
	}
}

func TestResolveDynamicQuery_nilExtension(t *testing.T) {
	t.Parallel()
	p := &Playlist{
		DPVersion: "1.1.0",
		Title:     "t",
		Items: []PlaylistItem{
			{Source: "https://static.example/a"},
		},
	}
	out, err := p.ResolveDynamicQuery(context.Background(), nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(out.Items) != 1 || out.Items[0].Source != "https://static.example/a" {
		t.Fatalf("%+v", out.Items)
	}
	if out == p {
		t.Fatal("expected new playlist value")
	}
}

func TestResolveDynamicQuery_httpsJSONGET(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("method %s", r.Method)
		}
		if r.URL.RawQuery != "chain=ethereum&owner=0xabc" {
			t.Errorf("query %q", r.URL.RawQuery)
		}
		_, _ = io.WriteString(w, `{"artworks":[{"id":"385f79b6-a45f-4c1c-8080-e93a192adccc","title":"T","source":"https://media.example/x"}]}`)
	}))
	t.Cleanup(srv.Close)

	p := &Playlist{
		DPVersion: "1.1.0",
		Title:     "t",
		Items:     []PlaylistItem{{Source: "https://first"}},
		DynamicQuery: &playlists.DynamicQuery{
			Profile:  ProfileHTTPSJSONV1,
			Endpoint: srv.URL + "/artworks",
			Method:   http.MethodGet,
			Query:    "chain=ethereum&owner={{owner}}",
			ResponseMapping: playlists.ResponseMapping{
				ItemsPath:  "artworks",
				ItemSchema: "dp1/1.1",
			},
		},
	}
	out, err := p.ResolveDynamicQuery(context.Background(), HydrationParams{"owner": "0xabc"}, srv.Client())
	if err != nil {
		t.Fatal(err)
	}
	if len(out.Items) != 2 {
		t.Fatalf("got %d items %+v", len(out.Items), out.Items)
	}
	if out.Items[0].Source != "https://first" {
		t.Fatal(out.Items[0].Source)
	}
	if out.Items[1].Title != "T" || out.Items[1].Source != "https://media.example/x" {
		t.Fatalf("%+v", out.Items[1])
	}
}

func TestResolveDynamicQuery_httpsJSONGET_noQueryString(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.RawQuery != "" {
			t.Errorf("expected no query, got %q", r.URL.RawQuery)
		}
		_, _ = io.WriteString(w, `{"items":[{"id":"385f79b6-a45f-4c1c-8080-e93a192adccc","title":"X","source":"https://y"}]}`)
	}))
	t.Cleanup(srv.Close)

	p := &Playlist{
		DPVersion: "1.1.0",
		Title:     "t",
		Items:     []PlaylistItem{{Source: "https://first"}},
		DynamicQuery: &playlists.DynamicQuery{
			Profile:  ProfileHTTPSJSONV1,
			Endpoint: srv.URL,
			Query:    "",
			ResponseMapping: playlists.ResponseMapping{
				ItemsPath:  "items",
				ItemSchema: "dp1/1.1",
			},
		},
	}
	out, err := p.ResolveDynamicQuery(context.Background(), HydrationParams{"unused": "x"}, srv.Client())
	if err != nil {
		t.Fatal(err)
	}
	if len(out.Items) != 2 || out.Items[1].Title != "X" {
		t.Fatalf("%+v", out.Items)
	}
}

func TestResolveDynamicQuery_graphQL(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("method %s", r.Method)
		}
		var body struct {
			Query string `json:"query"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatal(err)
		}
		wantSub := `ownedWorks(address: "0xw")`
		if !strings.Contains(body.Query, wantSub) {
			t.Fatalf("query %q", body.Query)
		}
		_, _ = io.WriteString(w, `{"data":{"ownedWorks":[{"id":"385f79b6-a45f-4c1c-8080-e93a192adccc","title":"G","source":"https://z"}]}}`)
	}))
	t.Cleanup(srv.Close)

	p := &Playlist{
		DPVersion: "1.1.0",
		Title:     "t",
		Items:     []PlaylistItem{{Source: "https://static"}},
		DynamicQuery: &playlists.DynamicQuery{
			Profile:  ProfileGraphQLV1,
			Endpoint: srv.URL,
			Query:    `query { ownedWorks(address: "{{addr}}") { id title source } }`,
			ResponseMapping: playlists.ResponseMapping{
				ItemsPath:  "data.ownedWorks",
				ItemSchema: "dp1/1.1",
			},
		},
	}
	out, err := p.ResolveDynamicQuery(context.Background(), HydrationParams{"addr": "0xw"}, srv.Client())
	if err != nil {
		t.Fatal(err)
	}
	if len(out.Items) != 2 || out.Items[1].Title != "G" {
		t.Fatalf("%+v", out.Items)
	}
}

func TestResolveDynamicQuery_itemMap(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.WriteString(w, `{"rows":[{"artwork_id":"385f79b6-a45f-4c1c-8080-e93a192adccc","name":"N","media_url":"https://u"}]}`)
	}))
	t.Cleanup(srv.Close)

	p := &Playlist{
		DPVersion: "1.1.0",
		Title:     "t",
		Items:     []PlaylistItem{{Source: "https://a"}},
		DynamicQuery: &playlists.DynamicQuery{
			Profile:  ProfileHTTPSJSONV1,
			Endpoint: srv.URL,
			ResponseMapping: playlists.ResponseMapping{
				ItemsPath:  "rows",
				ItemSchema: "dp1/1.1",
				ItemMap: map[string]string{
					"id":     "artwork_id",
					"title":  "name",
					"source": "media_url",
				},
			},
		},
	}
	out, err := p.ResolveDynamicQuery(context.Background(), nil, srv.Client())
	if err != nil {
		t.Fatal(err)
	}
	if len(out.Items) != 2 || out.Items[1].Title != "N" || out.Items[1].Source != "https://u" {
		t.Fatalf("%+v", out.Items[1])
	}
}

func TestResolveDynamicQuery_graphQL_errors(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.WriteString(w, `{"errors":[{"message":"boom"}]}`)
	}))
	t.Cleanup(srv.Close)

	p := &Playlist{
		DPVersion: "1.1.0",
		Title:     "t",
		Items:     []PlaylistItem{{Source: "https://a"}},
		DynamicQuery: &playlists.DynamicQuery{
			Profile:  ProfileGraphQLV1,
			Endpoint: srv.URL,
			Query:    "",
			ResponseMapping: playlists.ResponseMapping{
				ItemsPath:  "data.x",
				ItemSchema: "dp1/1.1",
			},
		},
	}
	_, err := p.ResolveDynamicQuery(context.Background(), nil, srv.Client())
	if err == nil || !errors.Is(err, ErrDynamicQueryResponse) {
		t.Fatalf("got %v", err)
	}
}

func TestResolveDynamicQuery_invalidPlaylistItem(t *testing.T) {
	t.Parallel()

	t.Run("missing_required_source", func(t *testing.T) {
		t.Parallel()
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			_, _ = io.WriteString(w, `{"artworks":[{"id":"385f79b6-a45f-4c1c-8080-e93a192adccc","title":"T"}]}`)
		}))
		t.Cleanup(srv.Close)

		p := &Playlist{
			DPVersion: "1.1.0",
			Title:     "t",
			Items:     []PlaylistItem{{Source: "https://static.example/ok"}},
			DynamicQuery: &playlists.DynamicQuery{
				Profile:  ProfileHTTPSJSONV1,
				Endpoint: srv.URL,
				ResponseMapping: playlists.ResponseMapping{
					ItemsPath:  "artworks",
					ItemSchema: "dp1/1.1",
				},
			},
		}
		_, err := p.ResolveDynamicQuery(context.Background(), nil, srv.Client())
		if !errors.Is(err, ErrDynamicQueryItemInvalid) {
			t.Fatalf("want ErrDynamicQueryItemInvalid, got %v", err)
		}
		if !errors.Is(err, validate.ErrValidation) {
			t.Fatalf("want wrapped validate.ErrValidation, got %v", err)
		}
	})

	t.Run("invalid_source_uri", func(t *testing.T) {
		t.Parallel()
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			_, _ = io.WriteString(w, `{"artworks":[{"title":"T","source":"not a uri"}]}`)
		}))
		t.Cleanup(srv.Close)

		p := &Playlist{
			DPVersion: "1.1.0",
			Title:     "t",
			Items:     []PlaylistItem{{Source: "https://static.example/ok"}},
			DynamicQuery: &playlists.DynamicQuery{
				Profile:  ProfileHTTPSJSONV1,
				Endpoint: srv.URL,
				ResponseMapping: playlists.ResponseMapping{
					ItemsPath:  "artworks",
					ItemSchema: "dp1/1.1",
				},
			},
		}
		_, err := p.ResolveDynamicQuery(context.Background(), nil, srv.Client())
		if !errors.Is(err, ErrDynamicQueryItemInvalid) {
			t.Fatalf("want ErrDynamicQueryItemInvalid, got %v", err)
		}
		if !errors.Is(err, validate.ErrValidation) {
			t.Fatalf("want wrapped validate.ErrValidation, got %v", err)
		}
	})

	t.Run("itemMap_produces_invalid_item", func(t *testing.T) {
		t.Parallel()
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			_, _ = io.WriteString(w, `{"artworks":[{"name":"N","media_url":"not a uri"}]}`)
		}))
		t.Cleanup(srv.Close)

		p := &Playlist{
			DPVersion: "1.1.0",
			Title:     "t",
			Items:     []PlaylistItem{{Source: "https://static.example/ok"}},
			DynamicQuery: &playlists.DynamicQuery{
				Profile:  ProfileHTTPSJSONV1,
				Endpoint: srv.URL,
				ResponseMapping: playlists.ResponseMapping{
					ItemsPath:  "artworks",
					ItemSchema: "dp1/1.1",
					ItemMap: map[string]string{
						"title":  "name",
						"source": "media_url",
					},
				},
			},
		}
		_, err := p.ResolveDynamicQuery(context.Background(), nil, srv.Client())
		if !errors.Is(err, ErrDynamicQueryItemInvalid) {
			t.Fatalf("want ErrDynamicQueryItemInvalid, got %v", err)
		}
		if !errors.Is(err, validate.ErrValidation) {
			t.Fatalf("want wrapped validate.ErrValidation, got %v", err)
		}
	})
}
