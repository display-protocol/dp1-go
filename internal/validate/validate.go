// Package validate loads embedded JSON Schema (draft 2020-12) documents and validates raw JSON payloads.
package validate

import (
	"encoding/json"
	"fmt"
	"io/fs"
	"path/filepath"
	"sync"

	jsonschema "github.com/santhosh-tekuri/jsonschema/v6"

	"github.com/display-protocol/dp1-go/internal/schema"
)

const (
	playlistSchemaURL                 = "https://dp1.feralfile.com/schemas/v1.1.1/playlist.json"
	playlistItemSchemaURL             = playlistSchemaURL + "#/$defs/PlaylistItem"
	playlistGroupSchemaURL            = "https://dp1.feralfile.com/schemas/v1.1.1/playlist-group.json"
	refManifestSchemaURL              = "https://dp1.feralfile.com/schemas/v1.1.1/ref-manifest.json"
	channelsExtensionSchemaURL        = "https://dp1.feralfile.com/extensions/channels/v0.1.0/schema.json"
	playlistsExtensionSchemaURL       = "https://dp1.feralfile.com/extensions/playlists/v0.1.0/schema.json"
	playlistPlaylistsOverlaySchemaURL = "https://display-protocol.github.io/dp1-go/schemas/overlay/playlist_and_playlists_extension.json"
)

var (
	compilerOnce sync.Once
	compiler     *jsonschema.Compiler
	errCompiler  error
	// jsonschema.Compiler.Compile is not safe for concurrent use on the same instance.
	compileMu sync.Mutex
)

func compilerSingleton() (*jsonschema.Compiler, error) {
	compilerOnce.Do(func() {
		c := jsonschema.NewCompiler()
		c.DefaultDraft(jsonschema.Draft2020)
		c.AssertFormat()

		walkErr := fs.WalkDir(schema.FS, ".", func(path string, d fs.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if d.IsDir() || filepath.Ext(path) != ".json" {
				return nil
			}
			b, err := schema.FS.ReadFile(path)
			if err != nil {
				return err
			}
			var meta struct {
				ID string `json:"$id"`
			}
			if err := json.Unmarshal(b, &meta); err != nil {
				return fmt.Errorf("schema %q: %w", path, err)
			}
			if meta.ID == "" {
				return fmt.Errorf("schema %q: missing $id", path)
			}
			var doc any
			if err := json.Unmarshal(b, &doc); err != nil {
				return fmt.Errorf("schema %q: %w", path, err)
			}
			if err := c.AddResource(meta.ID, doc); err != nil {
				return fmt.Errorf("schema %q: %w", path, err)
			}
			return nil
		})
		if walkErr != nil {
			errCompiler = walkErr
			return
		}
		compiler = c
	})
	return compiler, errCompiler
}

func validateAgainst(url string, data []byte) error {
	c, err := compilerSingleton()
	if err != nil {
		return err
	}
	var v any
	if err := json.Unmarshal(data, &v); err != nil {
		return fmt.Errorf("%w: json: %w", ErrValidation, err)
	}
	compileMu.Lock()
	defer compileMu.Unlock()
	sch, err := c.Compile(url)
	if err != nil {
		return fmt.Errorf("compile schema %s: %w", url, err)
	}
	if err := sch.Validate(v); err != nil {
		return fmt.Errorf("%w: %w", ErrValidation, err)
	}
	return nil
}

// Playlist validates a DP-1 playlist JSON document (core schema only).
func Playlist(data []byte) error {
	return validateAgainst(playlistSchemaURL, data)
}

// PlaylistItem validates raw JSON for a single playlist item against the core
// schema's PlaylistItem definition.
func PlaylistItem(data []byte) error {
	return validateAgainst(playlistItemSchemaURL, data)
}

// PlaylistWithPlaylistsExtension validates a playlist JSON document that may include
// the registered "playlists" extension fields (core + extension allOf).
func PlaylistWithPlaylistsExtension(data []byte) error {
	return validateAgainst(playlistPlaylistsOverlaySchemaURL, data)
}

// PlaylistGroup validates a DP-1 playlist-group (exhibition) document.
func PlaylistGroup(data []byte) error {
	return validateAgainst(playlistGroupSchemaURL, data)
}

// RefManifest validates a ref manifest JSON document.
func RefManifest(data []byte) error {
	return validateAgainst(refManifestSchemaURL, data)
}

// ChannelsExtension validates a channel extension document.
func ChannelsExtension(data []byte) error {
	return validateAgainst(channelsExtensionSchemaURL, data)
}

// PlaylistsExtensionFragment validates a JSON value against the playlists extension schema alone.
// Useful for partial documents; most callers should use PlaylistWithPlaylistsExtension on the full playlist.
func PlaylistsExtensionFragment(data []byte) error {
	return validateAgainst(playlistsExtensionSchemaURL, data)
}
