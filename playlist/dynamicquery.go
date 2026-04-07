package playlist

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"maps"
	"net"
	"net/http"
	"net/netip"
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
	// ErrDynamicQueryEndpointPolicy is returned when the outbound URL fails security policy
	// (scheme, userinfo, fragment, or SSRF checks). See [DynamicQueryFetchOptions].
	ErrDynamicQueryEndpointPolicy = errors.New("dynamicQuery: endpoint policy")
)

// DynamicQueryFetchOptions configures outbound dynamicQuery HTTP behavior.
// A nil pointer is treated as the zero value (secure defaults).
type DynamicQueryFetchOptions struct {
	// AllowInsecureHTTP, if true, permits http:// URLs and skips SSRF checks against
	// loopback, private, and link-local addresses (for example httptest servers).
	// When false (default), only https:// is allowed and the endpoint host must resolve
	// only to publicly routable addresses.
	AllowInsecureHTTP bool
}

var placeholderRE = regexp.MustCompile(`\{\{([a-zA-Z_][a-zA-Z0-9_]*)\}\}`)

// ResolveDynamicQuery resolves the playlists extension dynamicQuery on p, when present.
// It calls [PlaylistItemsFromDynamicQuery] with p.DynamicQuery and appends the returned items
// after any existing static items. The receiver is not modified; a new [Playlist] value is returned.
//
// If p.DynamicQuery is nil, returns a clone of p with no network I/O. If p is nil, returns an
// error wrapping [ErrDynamicQueryRequest].
//
// Hydration rules, HTTP behavior, and errors are the same as [PlaylistItemsFromDynamicQuery]
// with dq set to p.DynamicQuery.
//
// opts may be nil for default endpoint policy ([DynamicQueryFetchOptions] zero value).
func (p *Playlist) ResolveDynamicQuery(ctx context.Context, params HydrationParams, client *http.Client, opts *DynamicQueryFetchOptions) (*Playlist, error) {
	if p == nil {
		return nil, fmt.Errorf("%w: nil playlist", ErrDynamicQueryRequest)
	}
	out := clonePlaylist(p)
	if p.DynamicQuery == nil {
		return out, nil
	}

	dynamicItems, err := PlaylistItemsFromDynamicQuery(ctx, p.DynamicQuery, params, client, opts)
	if err != nil {
		return nil, err
	}

	merged := make([]PlaylistItem, 0, len(out.Items)+len(dynamicItems))
	merged = append(merged, out.Items...)
	merged = append(merged, dynamicItems...)
	out.Items = merged
	return out, nil
}

// PlaylistItemsFromDynamicQuery fetches and decodes dynamic playlist items from an indexer
// according to dq (the playlists extension dynamicQuery). It replaces {{name}} placeholders
// in dq.Query with params, issues one HTTP request to dq.Endpoint, walks dq.ResponseMapping
// (itemsPath, itemMap) to obtain objects, validates each against the core playlist item schema,
// and returns the decoded [PlaylistItem] slice. [Playlist.ResolveDynamicQuery] uses this
// function when p.DynamicQuery is non-nil.
//
// dq must be non-nil. ctx is attached to the outgoing request and DNS resolution for SSRF
// checks. client may be nil to use [http.DefaultClient]. opts may be nil for default
// endpoint policy ([DynamicQueryFetchOptions] zero value).
//
// When dq.Query is empty, no template hydration runs; the endpoint is still called (GraphQL
// POST sends an empty query string; https-json GET uses the bare endpoint URL; https-json POST
// may send an empty body).
//
// When dq.Query is non-empty, params must supply every placeholder name it uses; otherwise
// hydration fails with [ErrDynamicQueryHydration]. Extra keys in params are ignored.
func PlaylistItemsFromDynamicQuery(ctx context.Context, dq *playlists.DynamicQuery, params HydrationParams, client *http.Client, opts *DynamicQueryFetchOptions) ([]PlaylistItem, error) {
	if dq == nil {
		return nil, fmt.Errorf("%w: nil dynamicQuery", ErrDynamicQueryRequest)
	}
	body, err := fetchDynamicQueryResponseBody(ctx, dq, params, client, opts)
	if err != nil {
		return nil, err
	}
	return playlistItemsFromDynamicQueryBody(body, dq)
}

func fetchDynamicQueryResponseBody(ctx context.Context, dq *playlists.DynamicQuery, params HydrationParams, client *http.Client, opts *DynamicQueryFetchOptions) ([]byte, error) {
	if client == nil {
		client = http.DefaultClient
	}
	hq, err := hydrateDynamicQueryString(dq.Query, params)
	if err != nil {
		return nil, err
	}
	req, err := buildDynamicQueryRequest(ctx, dq, hq)
	if err != nil {
		return nil, err
	}
	if err := validateDynamicQueryRequestURL(ctx, req.URL, opts); err != nil {
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
	return body, nil
}

func playlistItemsFromDynamicQueryBody(body []byte, dq *playlists.DynamicQuery) ([]PlaylistItem, error) {
	if dq == nil {
		return nil, fmt.Errorf("%w: nil dynamicQuery", ErrDynamicQueryRequest)
	}
	switch dq.Profile {
	case ProfileHTTPSJSONV1, ProfileGraphQLV1:
	default:
		return nil, fmt.Errorf("%w: %q", ErrDynamicQueryUnknownProfile, dq.Profile)
	}

	rawItems, err := extractDynamicItems(body, dq.Profile, dq.ResponseMapping)
	if err != nil {
		return nil, err
	}

	out := make([]PlaylistItem, 0, len(rawItems))
	for _, raw := range rawItems {
		itemJSON, err := applyItemMap(raw, dq.ResponseMapping.ItemMap)
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
		out = append(out, it)
	}
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

func validateDynamicQueryRequestURL(ctx context.Context, u *url.URL, opts *DynamicQueryFetchOptions) error {
	if u == nil {
		return fmt.Errorf("%w: nil URL", ErrDynamicQueryEndpointPolicy)
	}
	if !u.IsAbs() || u.Host == "" {
		return fmt.Errorf("%w: URL must be absolute with a host", ErrDynamicQueryEndpointPolicy)
	}
	if u.User != nil {
		return fmt.Errorf("%w: URL must not include user info", ErrDynamicQueryEndpointPolicy)
	}
	if u.Fragment != "" {
		return fmt.Errorf("%w: URL must not include a fragment", ErrDynamicQueryEndpointPolicy)
	}

	allowInsecure := opts != nil && opts.AllowInsecureHTTP
	switch u.Scheme {
	case "https":
		// ok
	case "http":
		if !allowInsecure {
			return fmt.Errorf("%w: only https is allowed (set AllowInsecureHTTP for http)", ErrDynamicQueryEndpointPolicy)
		}
	default:
		return fmt.Errorf("%w: unsupported scheme %q", ErrDynamicQueryEndpointPolicy, u.Scheme)
	}
	if allowInsecure {
		return nil
	}

	host := u.Hostname()
	if host == "" {
		return fmt.Errorf("%w: missing host", ErrDynamicQueryEndpointPolicy)
	}
	if ip, err := netip.ParseAddr(host); err == nil {
		if !endpointIPAllowedProduction(ip) {
			return fmt.Errorf("%w: non-public endpoint address", ErrDynamicQueryEndpointPolicy)
		}
		return nil
	}
	return validateDNSHostProduction(ctx, host)
}

func endpointIPAllowedProduction(addr netip.Addr) bool {
	if !addr.IsValid() {
		return false
	}
	if addr.IsLoopback() || addr.IsPrivate() || addr.IsUnspecified() || addr.IsMulticast() || addr.IsLinkLocalUnicast() {
		return false
	}
	return true
}

func validateDNSHostProduction(ctx context.Context, host string) error {
	addrs, err := net.DefaultResolver.LookupIPAddr(ctx, host)
	if err != nil {
		return fmt.Errorf("%w: resolve host: %w", ErrDynamicQueryEndpointPolicy, err)
	}
	if len(addrs) == 0 {
		return fmt.Errorf("%w: host has no addresses", ErrDynamicQueryEndpointPolicy)
	}
	for _, ia := range addrs {
		ipAddr, err := netip.ParseAddr(ia.IP.String())
		if err != nil {
			return fmt.Errorf("%w: invalid resolved IP", ErrDynamicQueryEndpointPolicy)
		}
		if !endpointIPAllowedProduction(ipAddr) {
			return fmt.Errorf("%w: host resolves to non-public address", ErrDynamicQueryEndpointPolicy)
		}
	}
	return nil
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
