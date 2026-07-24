package displayat

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/display-protocol/dp1-go/extension/playlists"
	"github.com/display-protocol/dp1-go/playlist"
)

func strPtr(s string) *string { return &s }

func makeItem(id, title, source, displayAt string) playlist.PlaylistItem {
	it := playlist.PlaylistItem{
		ID:     id,
		Title:  title,
		Source: source,
	}
	if displayAt != "" {
		it.DisplayAt = strPtr(displayAt)
	}
	return it
}

// scheduledPlaylist builds a playlist with items for displayAt scheduling tests (§3.5.1).
func scheduledPlaylist(items ...playlist.PlaylistItem) *playlist.Playlist {
	return &playlist.Playlist{
		Items: items,
	}
}

func TestComputeActiveSet_BasicDaily(t *testing.T) {
	t.Parallel()

	p := scheduledPlaylist(
		makeItem("0", "Intro", "https://cdn.example.com/intro.html", ""),
		makeItem("1", "Day 1", "https://cdn.example.com/day1.html", "2026-07-21T00:00:00"),
		makeItem("2", "Day 2", "https://cdn.example.com/day2.html", "2026-07-22T00:00:00"),
		makeItem("3", "Day 3", "https://cdn.example.com/day3.html", "2026-07-23T00:00:00"),
		makeItem("4", "Outro", "https://cdn.example.com/outro.html", ""),
	)

	loc := time.UTC

	tests := []struct {
		name    string
		now     time.Time
		wantIDs []string
	}{
		{
			name:    "pre-Day1",
			now:     time.Date(2026, 7, 20, 10, 0, 0, 0, loc),
			wantIDs: []string{"0", "4"}, // only evergreen
		},
		{
			name:    "Day1 10am",
			now:     time.Date(2026, 7, 21, 10, 0, 0, 0, loc),
			wantIDs: []string{"0", "1", "4"},
		},
		{
			name:    "Day2 midnight exact",
			now:     time.Date(2026, 7, 22, 0, 0, 0, 0, loc),
			wantIDs: []string{"0", "2", "4"},
		},
		{
			name:    "Day2 afternoon",
			now:     time.Date(2026, 7, 22, 14, 0, 0, 0, loc),
			wantIDs: []string{"0", "2", "4"},
		},
		{
			name:    "Day3",
			now:     time.Date(2026, 7, 23, 12, 0, 0, 0, loc),
			wantIDs: []string{"0", "3", "4"},
		},
		{
			name:    "post-Day3",
			now:     time.Date(2026, 7, 24, 10, 0, 0, 0, loc),
			wantIDs: []string{"0", "3", "4"}, // Day3 is max past
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			active := ComputeActiveSet(p, tc.now, loc)
			gotIDs := make([]string, len(active))
			for i, it := range active {
				gotIDs[i] = it.ID
			}
			if len(gotIDs) != len(tc.wantIDs) {
				t.Fatalf("got %v, want %v", gotIDs, tc.wantIDs)
			}
			for i, id := range gotIDs {
				if id != tc.wantIDs[i] {
					t.Errorf("index %d: got %s, want %s", i, id, tc.wantIDs[i])
				}
			}
		})
	}
}

func TestComputeActiveSet_MultipleItemsSameDisplayAt(t *testing.T) {
	t.Parallel()

	p := scheduledPlaylist(
		makeItem("0", "Intro", "https://a.com/intro", ""),
		makeItem("1", "Work A", "https://a.com/a", "2026-07-21T00:00:00"),
		makeItem("2", "Work B", "https://a.com/b", "2026-07-22T00:00:00"),
		makeItem("3", "Work C", "https://a.com/c", "2026-07-22T00:00:00"), // same as B
		makeItem("4", "Outro", "https://a.com/outro", ""),
		makeItem("5", "Work D", "https://a.com/d", "2026-07-23T00:00:00"),
	)

	loc := time.UTC
	now := time.Date(2026, 7, 22, 14, 0, 0, 0, loc)

	active := ComputeActiveSet(p, now, loc)
	wantIDs := []string{"0", "2", "3", "4"} // Intro, B, C, Outro

	gotIDs := make([]string, len(active))
	for i, it := range active {
		gotIDs[i] = it.ID
	}

	if len(gotIDs) != len(wantIDs) {
		t.Fatalf("got %v, want %v", gotIDs, wantIDs)
	}
	for i, id := range gotIDs {
		if id != wantIDs[i] {
			t.Errorf("index %d: got %s, want %s", i, id, wantIDs[i])
		}
	}
}

func TestSchedule_ManyItemsSameDSTGap(t *testing.T) {
	t.Parallel()

	loc, err := time.LoadLocation("America/New_York")
	if err != nil {
		t.Fatalf("LoadLocation: %v", err)
	}

	items := []playlist.PlaylistItem{
		makeItem("evergreen", "Evergreen", "https://a.com/evergreen", ""),
	}
	for i := 0; i < 256; i++ {
		id := fmt.Sprintf("gap-%03d", i)
		items = append(items, makeItem(id, "Gap item", "https://a.com/"+id, "2026-03-08T02:30:00"))
	}
	p := scheduledPlaylist(items...)

	now := time.Date(2026, 3, 8, 3, 1, 0, 0, loc)
	active := ComputeActiveSet(p, now, loc)
	if len(active) != len(items) {
		t.Fatalf("active count = %d, want %d", len(active), len(items))
	}
	for i, it := range active {
		if it.ID != items[i].ID {
			t.Fatalf("active[%d] = %s, want %s", i, it.ID, items[i].ID)
		}
	}

	beforeGap := time.Date(2026, 3, 8, 1, 0, 0, 0, loc)
	next := NextDisplayAt(p, beforeGap, loc)
	if next == nil {
		t.Fatal("NextDisplayAt = nil, want first instant after DST gap")
	}
	if got, want := next.In(loc).Format(time.RFC3339), "2026-03-08T03:00:00-04:00"; got != want {
		t.Fatalf("NextDisplayAt = %s, want %s", got, want)
	}
}

func TestComputeActiveSet_AllFuture(t *testing.T) {
	t.Parallel()

	p := scheduledPlaylist(
		makeItem("0", "Intro", "https://a.com/intro", ""),
		makeItem("1", "Future1", "https://a.com/f1", "2026-07-25T00:00:00"),
		makeItem("2", "Future2", "https://a.com/f2", "2026-07-26T00:00:00"),
	)

	loc := time.UTC
	now := time.Date(2026, 7, 20, 0, 0, 0, 0, loc)

	active := ComputeActiveSet(p, now, loc)
	// Only evergreen items.
	if len(active) != 1 || active[0].ID != "0" {
		t.Errorf("got %v, want only Intro", active)
	}
}

func TestComputeActiveSet_NoEvergreen(t *testing.T) {
	t.Parallel()

	p := scheduledPlaylist(
		makeItem("1", "Day1", "https://a.com/d1", "2026-07-21T00:00:00"),
		makeItem("2", "Day2", "https://a.com/d2", "2026-07-22T00:00:00"),
	)

	loc := time.UTC
	now := time.Date(2026, 7, 21, 10, 0, 0, 0, loc)

	active := ComputeActiveSet(p, now, loc)
	if len(active) != 1 || active[0].ID != "1" {
		t.Errorf("got %v, want only Day1", active)
	}
}

func TestComputeActiveSet_EmptyPlaylist(t *testing.T) {
	t.Parallel()

	p := scheduledPlaylist()
	active := ComputeActiveSet(p, time.Now(), time.UTC)
	if len(active) != 0 {
		t.Errorf("expected empty, got %v", active)
	}
}

func TestComputeActiveSet_InvalidDisplayAt(t *testing.T) {
	t.Parallel()

	emptyDA := ""
	p := scheduledPlaylist(
		makeItem("0", "Intro", "https://a.com/intro", ""),
		makeItem("1", "Invalid", "https://a.com/inv", "not-a-date"),
		playlist.PlaylistItem{
			ID:        "empty",
			Title:     "Empty",
			Source:    "https://a.com/empty",
			DisplayAt: &emptyDA, // present but unresolvable — not evergreen
		},
		makeItem("2", "Valid", "https://a.com/v", "2026-07-21T00:00:00"),
	)

	loc := time.UTC
	now := time.Date(2026, 7, 21, 10, 0, 0, 0, loc)

	active := ComputeActiveSet(p, now, loc)
	// Present but unresolvable displayAt is excluded — not evergreen (§3.5.5).
	wantIDs := []string{"0", "2"}

	gotIDs := make([]string, len(active))
	for i, it := range active {
		gotIDs[i] = it.ID
	}

	if len(gotIDs) != len(wantIDs) {
		t.Fatalf("got %v, want %v", gotIDs, wantIDs)
	}
	for i := range wantIDs {
		if gotIDs[i] != wantIDs[i] {
			t.Fatalf("got %v, want %v", gotIDs, wantIDs)
		}
	}
}

func TestComputeActiveSet_CalendarInvalidDisplayAt(t *testing.T) {
	t.Parallel()

	// Date-only and calendar-invalid local values are unresolvable: excluded, not evergreen (§3.5.5).
	p := scheduledPlaylist(
		makeItem("0", "Intro", "https://a.com/intro", ""),
		makeItem("1", "DateOnly", "https://a.com/dateonly", "2026-02-28"), // date-only rejected
		makeItem("2", "BadLocal", "https://a.com/badlocal", "2026-02-30T00:00:00"),
		makeItem("3", "Valid", "https://a.com/v", "2026-07-21T00:00:00"),
	)

	loc := time.UTC
	now := time.Date(2026, 7, 21, 10, 0, 0, 0, loc)

	active := ComputeActiveSet(p, now, loc)
	wantIDs := []string{"0", "3"}

	gotIDs := make([]string, len(active))
	for i, it := range active {
		gotIDs[i] = it.ID
	}

	if len(gotIDs) != len(wantIDs) {
		t.Fatalf("got %v, want %v", gotIDs, wantIDs)
	}
	for i := range wantIDs {
		if gotIDs[i] != wantIDs[i] {
			t.Fatalf("got %v, want %v", gotIDs, wantIDs)
		}
	}
}

func TestComputeActiveSet_AbsoluteTimezone(t *testing.T) {
	t.Parallel()

	// Two items with same instant but different wire representations.
	p := scheduledPlaylist(
		makeItem("0", "Intro", "https://a.com/intro", ""),
		makeItem("1", "UTC", "https://a.com/utc", "2026-07-21T00:00:00Z"),
		makeItem("2", "Offset", "https://a.com/off", "2026-07-21T07:00:00+07:00"), // same instant
	)

	loc := time.UTC
	now := time.Date(2026, 7, 21, 0, 0, 0, 0, loc)

	active := ComputeActiveSet(p, now, loc)
	// Both timed items have the same max instant, so both included.
	wantIDs := []string{"0", "1", "2"}

	gotIDs := make([]string, len(active))
	for i, it := range active {
		gotIDs[i] = it.ID
	}

	if len(gotIDs) != len(wantIDs) {
		t.Fatalf("got %v, want %v", gotIDs, wantIDs)
	}
	for i := range wantIDs {
		if gotIDs[i] != wantIDs[i] {
			t.Fatalf("got %v, want %v", gotIDs, wantIDs)
		}
	}
}

func TestComputeActiveSet_PreservesOrder(t *testing.T) {
	t.Parallel()

	p := scheduledPlaylist(
		makeItem("A", "A", "https://a.com/a", "2026-07-22T00:00:00"),
		makeItem("B", "B", "https://a.com/b", ""),
		makeItem("C", "C", "https://a.com/c", "2026-07-22T00:00:00"),
		makeItem("D", "D", "https://a.com/d", ""),
	)

	loc := time.UTC
	now := time.Date(2026, 7, 22, 10, 0, 0, 0, loc)

	active := ComputeActiveSet(p, now, loc)
	wantIDs := []string{"A", "B", "C", "D"}

	gotIDs := make([]string, len(active))
	for i, it := range active {
		gotIDs[i] = it.ID
	}

	for i, id := range gotIDs {
		if id != wantIDs[i] {
			t.Errorf("order mismatch at %d: got %s, want %s", i, id, wantIDs[i])
		}
	}
}

func TestComputeActiveSet_MixedAbsentPresentInvalid(t *testing.T) {
	t.Parallel()

	// Unit-level §3.5.5 / cohort rules on a constructed item list (not a dynamicQuery fetch):
	// absent → evergreen; present "" / calendar-invalid that matches wire pattern → not eligible;
	// future timed → not yet in active set but is a NextDisplayAt candidate.
	p := scheduledPlaylist(
		makeItem("static", "Day", "https://static.example/day", "2026-07-21T00:00:00Z"),
		makeItem("future", "Future", "https://example/a", "2099-01-01T00:00:00Z"),
		playlist.PlaylistItem{ID: "empty", Title: "Empty", Source: "https://example/b", DisplayAt: strPtr("")},
		makeItem("cal-invalid", "CalInvalid", "https://example/c", "2026-02-30T00:00:00"),
		makeItem("absent", "Absent", "https://example/d", ""),
	)
	now := time.Date(2026, 7, 21, 12, 0, 0, 0, time.UTC)
	active := ComputeActiveSet(p, now, time.UTC)
	wantIDs := []string{"static", "absent"}
	if len(active) != len(wantIDs) {
		t.Fatalf("got %d items %v, want %d %v", len(active), itemIDs(active), len(wantIDs), wantIDs)
	}
	for i, id := range wantIDs {
		if active[i].ID != id {
			t.Fatalf("index %d: got %s, want %s", i, active[i].ID, id)
		}
	}
	next := NextDisplayAt(p, now, time.UTC)
	if next == nil {
		t.Fatal("NextDisplayAt: want future displayAt as candidate")
	}
	wantNext := time.Date(2099, 1, 1, 0, 0, 0, 0, time.UTC)
	if !next.Equal(wantNext) {
		t.Fatalf("NextDisplayAt = %v, want %v", next, wantNext)
	}
}

func TestComputeActiveSet_EmptyWhenNothingQualifies(t *testing.T) {
	t.Parallel()

	loc := time.UTC
	now := time.Date(2026, 7, 21, 12, 0, 0, 0, loc)

	t.Run("all_future_no_evergreen", func(t *testing.T) {
		t.Parallel()
		p := scheduledPlaylist(
			makeItem("1", "F1", "https://a.com/f1", "2026-07-25T00:00:00Z"),
			makeItem("2", "F2", "https://a.com/f2", "2026-07-26T00:00:00Z"),
		)
		active := ComputeActiveSet(p, now, loc)
		if len(active) != 0 {
			t.Fatalf("got %v, want empty", itemIDs(active))
		}
	})

	t.Run("only_present_unresolvable", func(t *testing.T) {
		t.Parallel()
		p := scheduledPlaylist(
			playlist.PlaylistItem{ID: "empty", Source: "https://a.com/e", DisplayAt: strPtr("")},
			makeItem("bad", "Bad", "https://a.com/b", "2026-02-30T00:00:00"),
		)
		active := ComputeActiveSet(p, now, loc)
		if len(active) != 0 {
			t.Fatalf("got %v, want empty", itemIDs(active))
		}
	})
}

func TestResolveDynamicQuery_ThenSchedule_Section356(t *testing.T) {
	t.Parallel()

	// §3.5.6 end-to-end: hydrate via ResolveDynamicQuery, then schedule on the merged list.
	// Pattern-invalid displayAt fails the overlay at fetch time; calendar-invalid that matches
	// the wire pattern is accepted then excluded by ComputeActiveSet (§3.5.5).
	const (
		staticID = "aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa"
		pastID   = "bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbbbbb"
		futureID = "cccccccc-cccc-cccc-cccc-cccccccccccc"
		absentID = "dddddddd-dddd-dddd-dddd-dddddddddddd"
		calID    = "eeeeeeee-eeee-eeee-eeee-eeeeeeeeeeee"
	)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.WriteString(w, fmt.Sprintf(`{
  "items":[
    {"id":%q,"source":"https://dyn.example/past","displayAt":"2026-07-20T00:00:00Z"},
    {"id":%q,"source":"https://dyn.example/future","displayAt":"2099-01-01T00:00:00Z"},
    {"id":%q,"source":"https://dyn.example/absent"},
    {"id":%q,"source":"https://dyn.example/cal","displayAt":"2026-02-30T00:00:00"}
  ]
}`, pastID, futureID, absentID, calID))
	}))
	t.Cleanup(srv.Close)

	p := &playlist.Playlist{
		DPVersion: "1.1.0",
		Title:     "t",
		Items: []playlist.PlaylistItem{{
			ID:        staticID,
			Source:    "https://static.example/day",
			DisplayAt: strPtr("2026-07-21T00:00:00Z"),
		}},
		DynamicQuery: &playlists.DynamicQuery{
			Profile:  playlist.ProfileHTTPSJSONV1,
			Endpoint: srv.URL,
			ResponseMapping: playlists.ResponseMapping{
				ItemsPath:  "items",
				ItemSchema: "dp1/1.1",
			},
		},
	}
	opts := &playlist.DynamicQueryFetchOptions{AllowInsecureHTTP: true}
	out, err := p.ResolveDynamicQuery(context.Background(), nil, srv.Client(), opts)
	if err != nil {
		t.Fatal(err)
	}

	now := time.Date(2026, 7, 21, 12, 0, 0, 0, time.UTC)
	active := ComputeActiveSet(out, now, time.UTC)
	// Max past displayAt is static (2026-07-21); dyn-past is earlier; dyn-absent evergreen;
	// dyn-future not yet; dyn-cal-invalid excluded.
	wantIDs := []string{staticID, absentID}
	gotIDs := itemIDs(active)
	if len(gotIDs) != len(wantIDs) {
		t.Fatalf("active got %v, want %v (full items=%v)", gotIDs, wantIDs, itemIDs(out.Items))
	}
	for i := range wantIDs {
		if gotIDs[i] != wantIDs[i] {
			t.Fatalf("active got %v, want %v", gotIDs, wantIDs)
		}
	}

	next := NextDisplayAt(out, now, time.UTC)
	if next == nil {
		t.Fatal("NextDisplayAt: want dyn-future as candidate")
	}
	wantNext := time.Date(2099, 1, 1, 0, 0, 0, 0, time.UTC)
	if !next.Equal(wantNext) {
		t.Fatalf("NextDisplayAt = %v, want %v", next, wantNext)
	}
}

func itemIDs(items []playlist.PlaylistItem) []string {
	ids := make([]string, len(items))
	for i, it := range items {
		ids[i] = it.ID
	}
	return ids
}

func TestComputeActiveSet_NoDisplayAtItemsReturnsAll(t *testing.T) {
	t.Parallel()

	// §3.5.1: when no item has displayAt, every item is evergreen — return full list.
	p := &playlist.Playlist{
		Items: []playlist.PlaylistItem{
			makeItem("1", "A", "https://a.com/a", ""),
			makeItem("2", "B", "https://a.com/b", ""),
		},
	}
	now := time.Date(2026, 7, 22, 10, 0, 0, 0, time.UTC)
	active := ComputeActiveSet(p, now, time.UTC)
	if len(active) != 2 {
		t.Fatalf("no displayAt items: got %d items, want 2", len(active))
	}
	if active[0].ID != "1" || active[1].ID != "2" {
		t.Fatalf("order: got %q,%q want 1,2", active[0].ID, active[1].ID)
	}
}

func TestNextDisplayAt_NoDisplayAtItemsReturnsNil(t *testing.T) {
	t.Parallel()

	// §3.5.1: when no item has displayAt, there is no scheduling timer.
	p := &playlist.Playlist{
		Items: []playlist.PlaylistItem{
			makeItem("1", "A", "https://a.com/a", ""),
			makeItem("2", "B", "https://a.com/b", ""),
		},
	}
	now := time.Date(2026, 7, 22, 10, 0, 0, 0, time.UTC)
	if next := NextDisplayAt(p, now, time.UTC); next != nil {
		t.Fatalf("no displayAt items: got %v, want nil", next)
	}
}

func TestNextDisplayAt_Basic(t *testing.T) {
	t.Parallel()

	p := scheduledPlaylist(
		makeItem("0", "Intro", "https://a.com/intro", ""),
		makeItem("1", "Day1", "https://a.com/d1", "2026-07-21T00:00:00"),
		makeItem("2", "Day2", "https://a.com/d2", "2026-07-22T00:00:00"),
		makeItem("3", "Day3", "https://a.com/d3", "2026-07-23T00:00:00"),
	)

	loc := time.UTC

	tests := []struct {
		name string
		now  time.Time
		want string // RFC3339 or "nil"
	}{
		{
			name: "before all",
			now:  time.Date(2026, 7, 20, 0, 0, 0, 0, loc),
			want: "2026-07-21T00:00:00Z",
		},
		{
			name: "during Day1",
			now:  time.Date(2026, 7, 21, 10, 0, 0, 0, loc),
			want: "2026-07-22T00:00:00Z",
		},
		{
			name: "during Day2",
			now:  time.Date(2026, 7, 22, 10, 0, 0, 0, loc),
			want: "2026-07-23T00:00:00Z",
		},
		{
			name: "during Day3",
			now:  time.Date(2026, 7, 23, 10, 0, 0, 0, loc),
			want: "nil",
		},
		{
			name: "after all",
			now:  time.Date(2026, 7, 30, 0, 0, 0, 0, loc),
			want: "nil",
		},
		{
			name: "exact boundary - at Day2 instant",
			now:  time.Date(2026, 7, 22, 0, 0, 0, 0, loc),
			want: "2026-07-23T00:00:00Z", // Day2 is not > now
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			next := NextDisplayAt(p, tc.now, loc)
			if tc.want == "nil" {
				if next != nil {
					t.Errorf("got %v, want nil", next)
				}
				return
			}
			if next == nil {
				t.Fatalf("got nil, want %s", tc.want)
			}
			got := next.UTC().Format(time.RFC3339)
			if got != tc.want {
				t.Errorf("got %s, want %s", got, tc.want)
			}
		})
	}
}

func TestNextDisplayAt_NoTimedItems(t *testing.T) {
	t.Parallel()

	p := scheduledPlaylist(
		makeItem("0", "A", "https://a.com/a", ""),
		makeItem("1", "B", "https://a.com/b", ""),
	)

	next := NextDisplayAt(p, time.Now(), time.UTC)
	if next != nil {
		t.Errorf("got %v, want nil", next)
	}
}

func TestNextDisplayAt_InvalidDisplayAt(t *testing.T) {
	t.Parallel()

	p := scheduledPlaylist(
		makeItem("0", "Invalid", "https://a.com/inv", "not-valid"),
		makeItem("1", "Valid", "https://a.com/v", "2026-07-25T00:00:00"),
	)

	loc := time.UTC
	now := time.Date(2026, 7, 20, 0, 0, 0, 0, loc)

	next := NextDisplayAt(p, now, loc)
	if next == nil {
		t.Fatal("got nil, want valid next")
	}
	want := "2026-07-25T00:00:00Z"
	got := next.UTC().Format(time.RFC3339)
	if got != want {
		t.Errorf("got %s, want %s", got, want)
	}
}

func TestNextDisplayAt_ReturnsSmallest(t *testing.T) {
	t.Parallel()

	p := scheduledPlaylist(
		makeItem("1", "Later", "https://a.com/l", "2026-07-30T00:00:00"),
		makeItem("2", "Sooner", "https://a.com/s", "2026-07-25T00:00:00"),
		makeItem("3", "Middle", "https://a.com/m", "2026-07-27T00:00:00"),
	)

	loc := time.UTC
	now := time.Date(2026, 7, 20, 0, 0, 0, 0, loc)

	next := NextDisplayAt(p, now, loc)
	if next == nil {
		t.Fatal("got nil")
	}
	want := "2026-07-25T00:00:00Z"
	got := next.UTC().Format(time.RFC3339)
	if got != want {
		t.Errorf("got %s, want %s", got, want)
	}
}

func TestComputeActiveSet_LocalDatetimeTimezone(t *testing.T) {
	t.Parallel()

	p := scheduledPlaylist(
		makeItem("0", "Intro", "https://a.com/intro", ""),
		makeItem("1", "Day1", "https://a.com/d1", "2026-07-21T00:00:00"),
	)

	locVN, _ := time.LoadLocation("Asia/Ho_Chi_Minh") // UTC+7
	locNY, _ := time.LoadLocation("America/New_York") // UTC-4 (summer)

	// At 2026-07-21 03:00 UTC:
	// - In VN (UTC+7): it's 10:00 on Jul 21 → Day1 is active
	// - In NY (UTC-4): it's 23:00 on Jul 20 → Day1 not yet active
	now := time.Date(2026, 7, 21, 3, 0, 0, 0, time.UTC)

	activeVN := ComputeActiveSet(p, now, locVN)
	if len(activeVN) != 2 {
		t.Errorf("VN: expected 2 items (Intro + Day1), got %d", len(activeVN))
	}

	activeNY := ComputeActiveSet(p, now, locNY)
	if len(activeNY) != 1 {
		t.Errorf("NY: expected 1 item (Intro only), got %d", len(activeNY))
	}
}

func TestComputeActiveSet_NilLocation(t *testing.T) {
	t.Parallel()

	p := scheduledPlaylist(
		makeItem("0", "Intro", "https://a.com/intro", ""),
		makeItem("1", "Day1", "https://a.com/d1", "2026-07-21T00:00:00"),
	)

	now := time.Date(2026, 7, 21, 10, 0, 0, 0, time.UTC)

	// nil location should default to UTC.
	active := ComputeActiveSet(p, now, nil)
	if len(active) != 2 {
		t.Errorf("expected 2 items, got %d", len(active))
	}
}

func TestNextDisplayAt_NilLocation(t *testing.T) {
	t.Parallel()

	p := scheduledPlaylist(
		makeItem("0", "Intro", "https://a.com/intro", ""),
		makeItem("1", "Day1", "https://a.com/d1", "2026-07-25T00:00:00"),
	)

	now := time.Date(2026, 7, 20, 0, 0, 0, 0, time.UTC)

	// nil location should default to UTC.
	next := NextDisplayAt(p, now, nil)
	if next == nil {
		t.Fatal("got nil, want non-nil")
	}
	want := "2026-07-25T00:00:00Z"
	got := next.UTC().Format(time.RFC3339)
	if got != want {
		t.Errorf("got %s, want %s", got, want)
	}
}
