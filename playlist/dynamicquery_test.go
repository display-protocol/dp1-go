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

	"github.com/display-protocol/dp1-go/extension/identity"
	"github.com/display-protocol/dp1-go/extension/playlists"
	"github.com/display-protocol/dp1-go/internal/validate"
)

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) {
	return f(r)
}

type errReadCloser struct{}

func (errReadCloser) Read(p []byte) (int, error) { return 0, errors.New("read body") }
func (errReadCloser) Close() error               { return nil }

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

func TestResolveDynamicQuery_itemMap_itemsPathDotNotation(t *testing.T) {
	t.Parallel()
	// Nested envelope: items live at response.payload.entries; itemMap still maps top-level keys on each row object.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.WriteString(w, `{
  "response": {
    "payload": {
      "meta": { "version": 1 },
      "entries": [
        {
          "artwork_id": "385f79b6-a45f-4c1c-8080-e93a192adccc",
          "name": "Nested",
          "media_url": "https://media.example/nested"
        }
      ]
    }
  }
}`)
	}))
	t.Cleanup(srv.Close)

	p := &Playlist{
		DPVersion: "1.1.0",
		Title:     "t",
		Items:     []PlaylistItem{{Source: "https://static.example/a"}},
		DynamicQuery: &playlists.DynamicQuery{
			Profile:  ProfileHTTPSJSONV1,
			Endpoint: srv.URL,
			ResponseMapping: playlists.ResponseMapping{
				ItemsPath:  "response.payload.entries",
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
	if len(out.Items) != 2 {
		t.Fatalf("want 2 items, got %d %+v", len(out.Items), out.Items)
	}
	it := out.Items[1]
	if it.Title != "Nested" || it.Source != "https://media.example/nested" {
		t.Fatalf("got %+v", it)
	}
	if it.ID != "385f79b6-a45f-4c1c-8080-e93a192adccc" {
		t.Fatalf("id %+v", it.ID)
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

func TestResolveDynamicQuery_nilPlaylist(t *testing.T) {
	t.Parallel()
	_, err := (*Playlist)(nil).ResolveDynamicQuery(context.Background(), nil, http.DefaultClient)
	if err == nil || !errors.Is(err, ErrDynamicQueryRequest) {
		t.Fatalf("got %v", err)
	}
}

func TestResolveDynamicQuery_httpDoError(t *testing.T) {
	t.Parallel()
	p := &Playlist{
		DPVersion: "1.1.0",
		Title:     "t",
		Items:     []PlaylistItem{{Source: "https://s"}},
		DynamicQuery: &playlists.DynamicQuery{
			Profile:  ProfileHTTPSJSONV1,
			Endpoint: "http://example.invalid",
			ResponseMapping: playlists.ResponseMapping{
				ItemsPath:  "x",
				ItemSchema: "dp1/1.1",
			},
		},
	}
	client := &http.Client{Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
		return nil, errors.New("transport boom")
	})}
	_, err := p.ResolveDynamicQuery(context.Background(), nil, client)
	if !errors.Is(err, ErrDynamicQueryHTTP) {
		t.Fatalf("got %v", err)
	}
}

func TestResolveDynamicQuery_readBodyError(t *testing.T) {
	t.Parallel()
	p := &Playlist{
		DPVersion: "1.1.0",
		Title:     "t",
		Items:     []PlaylistItem{{Source: "https://s"}},
		DynamicQuery: &playlists.DynamicQuery{
			Profile:  ProfileHTTPSJSONV1,
			Endpoint: "http://example.test/ok",
			ResponseMapping: playlists.ResponseMapping{
				ItemsPath:  "x",
				ItemSchema: "dp1/1.1",
			},
		},
	}
	client := &http.Client{Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
		return &http.Response{
			StatusCode: http.StatusOK,
			Body:       errReadCloser{},
		}, nil
	})}
	_, err := p.ResolveDynamicQuery(context.Background(), nil, client)
	if !errors.Is(err, ErrDynamicQueryHTTP) || !strings.Contains(err.Error(), "read body") {
		t.Fatalf("got %v", err)
	}
}

func TestResolveDynamicQuery_httpNotSuccess(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	t.Cleanup(srv.Close)

	p := &Playlist{
		DPVersion: "1.1.0",
		Title:     "t",
		Items:     []PlaylistItem{{Source: "https://s"}},
		DynamicQuery: &playlists.DynamicQuery{
			Profile:  ProfileHTTPSJSONV1,
			Endpoint: srv.URL,
			ResponseMapping: playlists.ResponseMapping{
				ItemsPath:  "x",
				ItemSchema: "dp1/1.1",
			},
		},
	}
	_, err := p.ResolveDynamicQuery(context.Background(), nil, srv.Client())
	if !errors.Is(err, ErrDynamicQueryHTTP) {
		t.Fatalf("got %v", err)
	}
}

func TestResolveDynamicQuery_unknownProfile(t *testing.T) {
	t.Parallel()
	p := &Playlist{
		DPVersion: "1.1.0",
		Title:     "t",
		Items:     []PlaylistItem{{Source: "https://s"}},
		DynamicQuery: &playlists.DynamicQuery{
			Profile:  "unknown-profile",
			Endpoint: "http://example.test",
			ResponseMapping: playlists.ResponseMapping{
				ItemsPath:  "x",
				ItemSchema: "dp1/1.1",
			},
		},
	}
	_, err := p.ResolveDynamicQuery(context.Background(), nil, http.DefaultClient)
	if !errors.Is(err, ErrDynamicQueryUnknownProfile) {
		t.Fatalf("got %v", err)
	}
}

func TestHydrateDynamicQueryString_duplicatePlaceholders(t *testing.T) {
	t.Parallel()
	got, err := HydrateDynamicQueryString(`owner={{a}}&dup={{a}}`, HydrationParams{"a": "1"})
	if err != nil {
		t.Fatal(err)
	}
	if got != `owner=1&dup=1` {
		t.Fatal(got)
	}
}

func TestHydrateDynamicQueryString_nilParamsMissing(t *testing.T) {
	t.Parallel()
	_, err := HydrateDynamicQueryString(`x={{k}}`, nil)
	if !errors.Is(err, ErrDynamicQueryHydration) {
		t.Fatalf("got %v", err)
	}
}

func TestBuildDynamicQueryRequest_nilDynamicQuery(t *testing.T) {
	t.Parallel()
	_, err := buildDynamicQueryRequest(context.Background(), nil, "")
	if !errors.Is(err, ErrDynamicQueryRequest) {
		t.Fatalf("got %v", err)
	}
}

func TestResolveDynamicQuery_invalidJSONBody(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.WriteString(w, `not json`)
	}))
	t.Cleanup(srv.Close)

	p := &Playlist{
		DPVersion: "1.1.0",
		Title:     "t",
		Items:     []PlaylistItem{{Source: "https://s"}},
		DynamicQuery: &playlists.DynamicQuery{
			Profile:  ProfileHTTPSJSONV1,
			Endpoint: srv.URL,
			ResponseMapping: playlists.ResponseMapping{
				ItemsPath:  "items",
				ItemSchema: "dp1/1.1",
			},
		},
	}
	_, err := p.ResolveDynamicQuery(context.Background(), nil, srv.Client())
	if !errors.Is(err, ErrDynamicQueryResponse) {
		t.Fatalf("got %v", err)
	}
}

func TestResolveDynamicQuery_itemsPathNotArray(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.WriteString(w, `{"artworks":"not-array"}`)
	}))
	t.Cleanup(srv.Close)

	p := &Playlist{
		DPVersion: "1.1.0",
		Title:     "t",
		Items:     []PlaylistItem{{Source: "https://s"}},
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
	if !errors.Is(err, ErrDynamicQueryResponse) {
		t.Fatalf("got %v", err)
	}
}

func TestResolveDynamicQuery_itemsPathWrongType(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.WriteString(w, `{"a":{"b":1}}`)
	}))
	t.Cleanup(srv.Close)

	p := &Playlist{
		DPVersion: "1.1.0",
		Title:     "t",
		Items:     []PlaylistItem{{Source: "https://s"}},
		DynamicQuery: &playlists.DynamicQuery{
			Profile:  ProfileHTTPSJSONV1,
			Endpoint: srv.URL,
			ResponseMapping: playlists.ResponseMapping{
				ItemsPath:  "a.b",
				ItemSchema: "dp1/1.1",
			},
		},
	}
	_, err := p.ResolveDynamicQuery(context.Background(), nil, srv.Client())
	if !errors.Is(err, ErrDynamicQueryResponse) {
		t.Fatalf("got %v", err)
	}
}

func TestResolveDynamicQuery_graphQLEmptyErrorMessage(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.WriteString(w, `{"errors":[{"message":""}]}`)
	}))
	t.Cleanup(srv.Close)

	p := &Playlist{
		DPVersion: "1.1.0",
		Title:     "t",
		Items:     []PlaylistItem{{Source: "https://s"}},
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
	if !errors.Is(err, ErrDynamicQueryResponse) || !strings.Contains(err.Error(), "graphql error") {
		t.Fatalf("got %v", err)
	}
}

func TestResolveDynamicQuery_graphQLInvalidJSON(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.WriteString(w, `{`)
	}))
	t.Cleanup(srv.Close)

	p := &Playlist{
		DPVersion: "1.1.0",
		Title:     "t",
		Items:     []PlaylistItem{{Source: "https://s"}},
		DynamicQuery: &playlists.DynamicQuery{
			Profile:  ProfileGraphQLV1,
			Endpoint: srv.URL,
			Query:    "",
			ResponseMapping: playlists.ResponseMapping{
				ItemsPath:  "data",
				ItemSchema: "dp1/1.1",
			},
		},
	}
	_, err := p.ResolveDynamicQuery(context.Background(), nil, srv.Client())
	if !errors.Is(err, ErrDynamicQueryResponse) {
		t.Fatalf("got %v", err)
	}
}

func TestResolveDynamicQuery_itemMapNonObjectElement(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.WriteString(w, `{"rows":[1,2]}`)
	}))
	t.Cleanup(srv.Close)

	p := &Playlist{
		DPVersion: "1.1.0",
		Title:     "t",
		Items:     []PlaylistItem{{Source: "https://s"}},
		DynamicQuery: &playlists.DynamicQuery{
			Profile:  ProfileHTTPSJSONV1,
			Endpoint: srv.URL,
			ResponseMapping: playlists.ResponseMapping{
				ItemsPath: "rows",
				ItemMap: map[string]string{
					"source": "url",
				},
				ItemSchema: "dp1/1.1",
			},
		},
	}
	_, err := p.ResolveDynamicQuery(context.Background(), nil, srv.Client())
	if !errors.Is(err, ErrDynamicQueryItemInvalid) {
		t.Fatalf("got %v", err)
	}
}

func TestResolveDynamicQuery_jsonUnmarshalItemFails(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Valid JSON for schema validation path but struct decode can still fail on unexpected types
		// if we bypass — use extreme value; playlist item expects object. Large number as item fails unmarshal into struct.
		_, _ = io.WriteString(w, `{"items":[null]}`)
	}))
	t.Cleanup(srv.Close)

	p := &Playlist{
		DPVersion: "1.1.0",
		Title:     "t",
		Items:     []PlaylistItem{{Source: "https://s"}},
		DynamicQuery: &playlists.DynamicQuery{
			Profile:  ProfileHTTPSJSONV1,
			Endpoint: srv.URL,
			ResponseMapping: playlists.ResponseMapping{
				ItemsPath:  "items",
				ItemSchema: "dp1/1.1",
			},
		},
	}
	_, err := p.ResolveDynamicQuery(context.Background(), nil, srv.Client())
	if !errors.Is(err, ErrDynamicQueryItemInvalid) {
		t.Fatalf("got %v", err)
	}
}

func TestResolveDynamicQuery_clonePlaylistBranches(t *testing.T) {
	t.Parallel()
	dur := 1.0
	margin := json.RawMessage(`"5%"`)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.WriteString(w, `{"items":[{"id":"385f79b6-a45f-4c1c-8080-e93a192adccc","title":"Z","source":"https://z"}]}`)
	}))
	t.Cleanup(srv.Close)

	orig := &Playlist{
		DPVersion: "1.1.0",
		Title:     "orig",
		Defaults: &Defaults{
			Display: &DisplayPrefs{
				Scaling: "fit",
				Margin:  margin,
			},
			License:  "MIT",
			Duration: &dur,
		},
		Items: []PlaylistItem{
			{
				Source:   "https://static",
				Override: json.RawMessage(`{"display":{"scaling":"fill"}}`),
			},
		},
		Signatures: []Signature{{Alg: AlgEd25519, Kid: "k", Ts: "t", PayloadHash: "h", Role: RoleCurator, Sig: "s"}},
		Curators:   []identity.Entity{{Name: "n", Key: "did:key:x"}},
		DynamicQuery: &playlists.DynamicQuery{
			Profile:  ProfileHTTPSJSONV1,
			Endpoint: srv.URL,
			Headers:  map[string]string{"X-Test": "1"},
			ResponseMapping: playlists.ResponseMapping{
				ItemsPath:  "items",
				ItemSchema: "dp1/1.1",
				ItemMap:    map[string]string{"title": "title"},
			},
		},
	}
	out, err := orig.ResolveDynamicQuery(context.Background(), nil, srv.Client())
	if err != nil {
		t.Fatal(err)
	}
	if out == orig {
		t.Fatal("expected distinct playlist pointer")
	}
	if out.Defaults == orig.Defaults || out.Defaults.Display == orig.Defaults.Display {
		t.Fatal("expected cloned defaults")
	}
	if len(out.Signatures) != len(orig.Signatures) || &out.Signatures[0] == &orig.Signatures[0] {
		t.Fatal("expected cloned signatures slice")
	}
	if len(out.Curators) != len(orig.Curators) || &out.Curators[0] == &orig.Curators[0] {
		t.Fatal("expected cloned curators")
	}
	if out.DynamicQuery == orig.DynamicQuery || out.DynamicQuery.Headers["X-Test"] != "1" {
		t.Fatal("expected cloned dynamicQuery")
	}
	if len(orig.Items[0].Override) > 0 {
		if &out.Items[0].Override[0] == &orig.Items[0].Override[0] {
			t.Fatal("expected cloned item override buffer")
		}
	}
	if len(out.Items) != 2 {
		t.Fatalf("items %+v", out.Items)
	}
}

func TestResolveDynamicQuery_httpsJSONPost(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("method %s", r.Method)
		}
		b, _ := io.ReadAll(r.Body)
		if string(b) != `{"filter":"a"}` {
			t.Errorf("body %q", b)
		}
		_, _ = io.WriteString(w, `{"items":[{"id":"385f79b6-a45f-4c1c-8080-e93a192adccc","title":"P","source":"https://p"}]}`)
	}))
	t.Cleanup(srv.Close)

	p := &Playlist{
		DPVersion: "1.1.0",
		Title:     "t",
		Items:     []PlaylistItem{{Source: "https://first"}},
		DynamicQuery: &playlists.DynamicQuery{
			Profile:  ProfileHTTPSJSONV1,
			Endpoint: srv.URL,
			Method:   http.MethodPost,
			Query:    `{"filter":"{{f}}"}`,
			ResponseMapping: playlists.ResponseMapping{
				ItemsPath:  "items",
				ItemSchema: "dp1/1.1",
			},
		},
	}
	out, err := p.ResolveDynamicQuery(context.Background(), HydrationParams{"f": "a"}, srv.Client())
	if err != nil {
		t.Fatal(err)
	}
	if len(out.Items) != 2 || out.Items[1].Title != "P" {
		t.Fatalf("%+v", out.Items)
	}
}
