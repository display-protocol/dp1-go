package playlist

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"maps"
	"net/http"
	"net/url"
	"regexp"
	"strings"

	"github.com/display-protocol/dp1-go/extension/identity"
	"github.com/display-protocol/dp1-go/extension/playlists"
	"github.com/display-protocol/dp1-go/internal/validate"
)

// Resolution profile values for DynamicQuery (playlists extension).
const (
	ProfileHTTPSJSONV1 = "https-json-v1"
	ProfileGraphQLV1   = "graphql-v1"
)

// HydrationParams maps template placeholder names (without "{{ }}") to replacement
// strings. Keys must match names inside templates, e.g. "viewer_address" for "{{viewer_address}}".
type HydrationParams map[string]string

var (
	// ErrDynamicQueryUnknownProfile is returned when DynamicQuery.profile is not supported.
	ErrDynamicQueryUnknownProfile = errors.New("dynamicQuery: unknown profile")
	// ErrDynamicQueryHydration is returned when a template placeholder cannot be satisfied from HydrationParams.
	ErrDynamicQueryHydration = errors.New("dynamicQuery: hydration")
	// ErrDynamicQueryRequest is returned when building the outbound HTTP request fails.
	ErrDynamicQueryRequest = errors.New("dynamicQuery: build request")
	// ErrDynamicQueryHTTP is returned for non-success HTTP status or empty body when errors prevent use of data.
	ErrDynamicQueryHTTP = errors.New("dynamicQuery: http")
	// ErrDynamicQueryResponse is returned when the indexer response is not usable (path, shape, GraphQL errors).
	ErrDynamicQueryResponse = errors.New("dynamicQuery: response")
	// ErrDynamicQueryItemInvalid is returned when a mapped item fails schema validation or decode.
	ErrDynamicQueryItemInvalid = errors.New("dynamicQuery: invalid playlist item")
)

var placeholderRE = regexp.MustCompile(`\{\{([a-zA-Z_][a-zA-Z0-9_]*)\}\}`)

// ResolveDynamicQuery performs one dynamic fetch when p.DynamicQuery is set, appends decoded
// playlist items after existing static items, and returns a new Playlist. The receiver is not modified.
//
// If DynamicQuery is nil, returns a copy of p with no network I/O.
//
// When DynamicQuery.query is empty, no template hydration runs; the endpoint is still called
// (GraphQL uses POST with "query": ""; https-json GET uses the bare endpoint URL; POST uses an empty body).
//
// params keys must cover every {{name}} in query when query is non-empty; otherwise hydration fails with
// ErrDynamicQueryHydration. Extra keys in params are ignored.
//
// client may be nil to use http.DefaultClient. ctx is attached to the outgoing request.
func (p *Playlist) ResolveDynamicQuery(ctx context.Context, params HydrationParams, client *http.Client) (*Playlist, error) {
	if p == nil {
		return nil, fmt.Errorf("%w: nil playlist", ErrDynamicQueryRequest)
	}
	out := clonePlaylist(p)
	if p.DynamicQuery == nil {
		return out, nil
	}
	if client == nil {
		client = http.DefaultClient
	}

	hq, err := hydrateDynamicQueryString(p.DynamicQuery.Query, params)
	if err != nil {
		return nil, err
	}
	req, err := buildDynamicQueryRequest(ctx, p.DynamicQuery, hq)
	if err != nil {
		return nil, err
	}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("%w: %w", ErrDynamicQueryHTTP, err)
	}
	defer func() { _ = resp.Body.Close() }()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("%w: read body: %w", ErrDynamicQueryHTTP, err)
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("%w: status %s", ErrDynamicQueryHTTP, resp.Status)
	}

	rawItems, err := extractDynamicItems(body, p.DynamicQuery.Profile, p.DynamicQuery.ResponseMapping)
	if err != nil {
		return nil, err
	}

	merged := make([]PlaylistItem, 0, len(out.Items)+len(rawItems))
	merged = append(merged, out.Items...)
	for _, raw := range rawItems {
		itemJSON, err := applyItemMap(raw, p.DynamicQuery.ResponseMapping.ItemMap)
		if err != nil {
			return nil, fmt.Errorf("%w: itemMap: %w", ErrDynamicQueryItemInvalid, err)
		}
		if err := validate.PlaylistItem(itemJSON); err != nil {
			return nil, fmt.Errorf("%w: %w", ErrDynamicQueryItemInvalid, err)
		}
		var it PlaylistItem
		if err := json.Unmarshal(itemJSON, &it); err != nil {
			return nil, fmt.Errorf("%w: %w", ErrDynamicQueryItemInvalid, err)
		}
		merged = append(merged, it)
	}
	out.Items = merged
	return out, nil
}

// HydrateDynamicQueryString replaces {{name}} placeholders using params. If query is empty,
// returns ("", nil) and does not inspect params.
func HydrateDynamicQueryString(query string, params HydrationParams) (string, error) {
	return hydrateDynamicQueryString(query, params)
}

func hydrateDynamicQueryString(query string, params HydrationParams) (string, error) {
	if query == "" {
		return "", nil
	}
	if params == nil {
		params = HydrationParams{}
	}
	seen := map[string]struct{}{}
	var missing []string
	for _, m := range placeholderRE.FindAllStringSubmatch(query, -1) {
		name := m[1]
		if _, ok := seen[name]; ok {
			continue
		}
		seen[name] = struct{}{}
		if _, ok := params[name]; !ok {
			missing = append(missing, name)
		}
	}
	if len(missing) > 0 {
		return "", fmt.Errorf("%w: missing params for placeholders: %s", ErrDynamicQueryHydration, strings.Join(missing, ", "))
	}

	var b strings.Builder
	last := 0
	for _, loc := range placeholderRE.FindAllStringSubmatchIndex(query, -1) {
		b.WriteString(query[last:loc[0]])
		name := query[loc[2]:loc[3]]
		b.WriteString(params[name])
		last = loc[1]
	}
	b.WriteString(query[last:])
	return b.String(), nil
}

func defaultHTTPMethod(profile, explicit string) string {
	if explicit != "" {
		return explicit
	}
	switch profile {
	case ProfileGraphQLV1:
		return http.MethodPost
	case ProfileHTTPSJSONV1:
		return http.MethodGet
	default:
		return http.MethodGet
	}
}

func buildDynamicQueryRequest(ctx context.Context, dq *playlists.DynamicQuery, hydratedQuery string) (*http.Request, error) {
	if dq == nil {
		return nil, fmt.Errorf("%w: nil dynamicQuery", ErrDynamicQueryRequest)
	}
	switch dq.Profile {
	case ProfileHTTPSJSONV1, ProfileGraphQLV1:
	default:
		return nil, fmt.Errorf("%w: %q", ErrDynamicQueryUnknownProfile, dq.Profile)
	}

	method := defaultHTTPMethod(dq.Profile, dq.Method)
	headers := http.Header{}
	for k, v := range dq.Headers {
		headers.Set(k, v)
	}

	switch dq.Profile {
	case ProfileGraphQLV1:
		if method != http.MethodPost {
			return nil, fmt.Errorf("%w: graphql-v1 expects POST", ErrDynamicQueryRequest)
		}
		if headers.Get("Content-Type") == "" {
			headers.Set("Content-Type", "application/json")
		}
		bodyObj := map[string]string{"query": hydratedQuery}
		raw, err := json.Marshal(bodyObj)
		if err != nil {
			return nil, fmt.Errorf("%w: %w", ErrDynamicQueryRequest, err)
		}
		req, err := http.NewRequestWithContext(ctx, method, dq.Endpoint, bytes.NewReader(raw))
		if err != nil {
			return nil, fmt.Errorf("%w: %w", ErrDynamicQueryRequest, err)
		}
		req.Header = headers
		return req, nil

	case ProfileHTTPSJSONV1:
		switch method {
		case http.MethodGet:
			u, err := url.Parse(dq.Endpoint)
			if err != nil {
				return nil, fmt.Errorf("%w: endpoint URL: %w", ErrDynamicQueryRequest, err)
			}
			if hydratedQuery != "" {
				extra, qerr := url.ParseQuery(hydratedQuery)
				if qerr != nil {
					return nil, fmt.Errorf("%w: query string: %w", ErrDynamicQueryRequest, qerr)
				}
				q2 := u.Query()
				for k, vs := range extra {
					for _, v := range vs {
						q2.Add(k, v)
					}
				}
				u.RawQuery = q2.Encode()
			}
			req, err := http.NewRequestWithContext(ctx, method, u.String(), nil)
			if err != nil {
				return nil, fmt.Errorf("%w: %w", ErrDynamicQueryRequest, err)
			}
			req.Header = headers
			return req, nil

		case http.MethodPost:
			var body io.Reader
			if hydratedQuery != "" {
				body = strings.NewReader(hydratedQuery)
			}
			req, err := http.NewRequestWithContext(ctx, method, dq.Endpoint, body)
			if err != nil {
				return nil, fmt.Errorf("%w: %w", ErrDynamicQueryRequest, err)
			}
			if hydratedQuery != "" && headers.Get("Content-Type") == "" {
				headers.Set("Content-Type", "application/json")
			}
			req.Header = headers
			return req, nil

		default:
			return nil, fmt.Errorf("%w: unsupported method %s for https-json-v1", ErrDynamicQueryRequest, method)
		}

	default:
		return nil, fmt.Errorf("%w: %q", ErrDynamicQueryUnknownProfile, dq.Profile)
	}
}

func graphqlPayload(body []byte) (data json.RawMessage, err error) {
	var env struct {
		Data   json.RawMessage `json:"data"`
		Errors []struct {
			Message string `json:"message"`
		} `json:"errors"`
	}
	if err := json.Unmarshal(body, &env); err != nil {
		return nil, fmt.Errorf("%w: json: %w", ErrDynamicQueryResponse, err)
	}
	if len(env.Errors) > 0 {
		msg := env.Errors[0].Message
		if msg == "" {
			msg = "graphql error"
		}
		return nil, fmt.Errorf("%w: graphql: %s", ErrDynamicQueryResponse, msg)
	}
	return env.Data, nil
}

func extractDynamicItems(body []byte, profile string, rm playlists.ResponseMapping) ([]json.RawMessage, error) {
	var root any
	if err := json.Unmarshal(body, &root); err != nil {
		return nil, fmt.Errorf("%w: json: %w", ErrDynamicQueryResponse, err)
	}

	if profile == ProfileGraphQLV1 {
		// Reject GraphQL error responses; walk itemsPath on the full envelope
		// (e.g. itemsPath "data.userWorks").
		if _, err := graphqlPayload(body); err != nil {
			return nil, err
		}
	}

	at, err := jsonAtDotPath(root, rm.ItemsPath)
	if err != nil {
		return nil, fmt.Errorf("%w: itemsPath %q: %w", ErrDynamicQueryResponse, rm.ItemsPath, err)
	}
	arr, ok := at.([]any)
	if !ok {
		return nil, fmt.Errorf("%w: itemsPath %q is not an array", ErrDynamicQueryResponse, rm.ItemsPath)
	}
	out := make([]json.RawMessage, 0, len(arr))
	for i, el := range arr {
		b, err := json.Marshal(el)
		if err != nil {
			return nil, fmt.Errorf("%w: item %d: %w", ErrDynamicQueryResponse, i, err)
		}
		out = append(out, b)
	}
	return out, nil
}

func jsonAtDotPath(root any, path string) (any, error) {
	parts := strings.Split(path, ".")
	cur := root
	for _, p := range parts {
		if p == "" {
			continue
		}
		m, ok := cur.(map[string]any)
		if !ok {
			return nil, fmt.Errorf("expected object at segment %q", p)
		}
		v, ok := m[p]
		if !ok {
			return nil, fmt.Errorf("missing key %q", p)
		}
		cur = v
	}
	return cur, nil
}

// applyItemMap maps indexer field names to DP-1 item fields: each key is the DP-1
// property name, each value is the source property name in the indexer JSON.
func applyItemMap(raw json.RawMessage, itemMap map[string]string) (json.RawMessage, error) {
	if len(itemMap) == 0 {
		return raw, nil
	}
	var obj map[string]json.RawMessage
	if err := json.Unmarshal(raw, &obj); err != nil {
		return nil, err
	}
	out := maps.Clone(obj)
	for dpKey, idxKey := range itemMap {
		if v, ok := obj[idxKey]; ok {
			out[dpKey] = v
			if idxKey != dpKey {
				delete(out, idxKey)
			}
		}
	}
	return json.Marshal(out)
}

func clonePlaylist(p *Playlist) *Playlist {
	c := *p
	if len(p.Items) > 0 {
		c.Items = append([]PlaylistItem(nil), p.Items...)
	}
	if len(p.Signatures) > 0 {
		c.Signatures = append([]Signature(nil), p.Signatures...)
	}
	if len(p.Curators) > 0 {
		c.Curators = append([]identity.Entity(nil), p.Curators...)
	}
	if p.Defaults != nil {
		d := *p.Defaults
		if p.Defaults.Display != nil {
			dp := *p.Defaults.Display
			d.Display = &dp
		}
		c.Defaults = &d
	}
	if p.DynamicQuery != nil {
		dq := *p.DynamicQuery
		if len(p.DynamicQuery.Headers) > 0 {
			dq.Headers = maps.Clone(p.DynamicQuery.Headers)
		}
		if len(p.DynamicQuery.ResponseMapping.ItemMap) > 0 {
			dq.ResponseMapping.ItemMap = maps.Clone(p.DynamicQuery.ResponseMapping.ItemMap)
		}
		c.DynamicQuery = &dq
	}
	for i := range c.Items {
		if len(c.Items[i].Override) > 0 {
			c.Items[i].Override = append(json.RawMessage(nil), c.Items[i].Override...)
		}
	}
	return &c
}
