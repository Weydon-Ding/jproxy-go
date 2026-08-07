package transmission

import (
	"context"
	"encoding/json"
	"path"
	"regexp"
	"strings"
	"unicode"
)

var videoAndSubtitleExtension = regexp.MustCompile(`(\.(mp4|avi|wmv|flv|mov|mkv|webm|mpg|mpeg|3gp|iso|ts|([-_a-zA-Z]{2,5}\.|)ass|([-_a-zA-Z]{2,5}\.|)srt|([-_a-zA-Z]{2,5}\.|)ssa|([-_a-zA-Z]{2,5}\.|)idx|([-_a-zA-Z]{2,5}\.|)sub))$`)

type Torrent struct {
	ID    int64
	Name  string
	Files []string
}

type torrentGetArguments struct {
	IDs    []string `json:"ids"`
	Fields []string `json:"fields"`
}

func (torrentGetArguments) requestArguments() {}

type torrentGetResponse struct {
	Torrents []torrentWire `json:"torrents"`
}

func (response *torrentGetResponse) decodeArguments(arguments json.RawMessage) error {
	var wire struct {
		Torrents json.RawMessage `json:"torrents"`
	}
	if err := json.Unmarshal(arguments, &wire); err != nil || wire.Torrents == nil || string(wire.Torrents) == "null" {
		return ErrMalformedResponse
	}
	return json.Unmarshal(wire.Torrents, &response.Torrents)
}

type torrentWire struct {
	ID    json.RawMessage `json:"id"`
	Name  *string         `json:"name"`
	Files *[]fileWire     `json:"files"`
}

type fileWire struct {
	Name string `json:"name"`
}

type sessionGetArguments struct {
	Fields []string `json:"fields"`
}

func (sessionGetArguments) requestArguments() {}

type sessionGetResponse struct {
	Version *string `json:"version"`
}

func (response *sessionGetResponse) decodeArguments(arguments json.RawMessage) error {
	if err := json.Unmarshal(arguments, response); err != nil || response.Version == nil || strings.TrimSpace(*response.Version) == "" {
		return ErrMalformedResponse
	}
	return nil
}

type renamePathArguments struct {
	IDs  []int64 `json:"ids"`
	Path string  `json:"path"`
	Name string  `json:"name"`
}

func (renamePathArguments) requestArguments() {}

type renamePathResponse struct {
	expectedPath string
	expectedName string
	expectedID   int64
}

func (response *renamePathResponse) decodeArguments(arguments json.RawMessage) error {
	var wire struct {
		Path *string         `json:"path"`
		Name *string         `json:"name"`
		ID   json.RawMessage `json:"id"`
	}
	if err := json.Unmarshal(arguments, &wire); err != nil || wire.Path == nil || wire.Name == nil || wire.ID == nil || string(wire.ID) == "null" || *wire.Path != response.expectedPath || *wire.Name != response.expectedName {
		return ErrMalformedResponse
	}
	var id int64
	if err := json.Unmarshal(wire.ID, &id); err != nil || id <= 0 || id != response.expectedID {
		return ErrMalformedResponse
	}
	return nil
}

func (c *Client) Torrent(ctx context.Context, hash string) (Torrent, error) {
	if invalidHash(hash) {
		return Torrent{}, ErrInvalidConfig
	}
	cfg, err := c.currentConfig()
	if err != nil {
		return Torrent{}, err
	}
	return c.torrent(ctx, cfg, hash)
}

func (c *Client) torrent(ctx context.Context, cfg config, hash string) (Torrent, error) {
	var response torrentGetResponse
	err := c.call(ctx, cfg, "torrent-get", torrentGetArguments{IDs: []string{hash}, Fields: []string{"id", "name", "files"}}, &response)
	if err != nil {
		return Torrent{}, err
	}
	if len(response.Torrents) == 0 {
		return Torrent{}, ErrUnknownTorrent
	}
	if len(response.Torrents) != 1 || response.Torrents[0].Name == nil || strings.TrimSpace(*response.Torrents[0].Name) == "" || response.Torrents[0].Files == nil {
		return Torrent{}, ErrMalformedResponse
	}
	var id int64
	if err := json.Unmarshal(response.Torrents[0].ID, &id); err != nil || id <= 0 {
		return Torrent{}, ErrMalformedResponse
	}
	torrent := Torrent{ID: id, Name: *response.Torrents[0].Name, Files: make([]string, len(*response.Torrents[0].Files))}
	for index, file := range *response.Torrents[0].Files {
		if strings.TrimSpace(file.Name) == "" {
			return Torrent{}, ErrMalformedResponse
		}
		torrent.Files[index] = file.Name
	}
	return torrent, nil
}

func (c *Client) Name(ctx context.Context, hash string) (string, error) {
	torrent, err := c.Torrent(ctx, hash)
	if err != nil {
		return "", err
	}
	return torrent.Name, nil
}

func (c *Client) Files(ctx context.Context, hash string) ([]string, error) {
	torrent, err := c.Torrent(ctx, hash)
	if err != nil {
		return nil, err
	}
	return torrent.Files, nil
}

func (c *Client) Rename(ctx context.Context, hash, name string) error {
	if invalidHash(hash) {
		return ErrInvalidConfig
	}
	if !validRenameName(name) {
		return ErrInvalidPath
	}
	cfg, err := c.currentConfig()
	if err != nil {
		return err
	}
	torrent, err := c.torrent(ctx, cfg, hash)
	if err != nil {
		return err
	}
	target := strings.ReplaceAll(name, ":", "_")
	if matches := videoAndSubtitleExtension.FindStringSubmatch(torrent.Name); len(matches) > 1 {
		target += matches[1]
	}
	if torrent.Name == target {
		return nil
	}
	return c.renamePath(ctx, cfg, torrent.ID, torrent.Name, target)
}

func (c *Client) RenameFile(ctx context.Context, hash, oldPath, newPath string) error {
	if invalidHash(hash) {
		return ErrInvalidConfig
	}
	if !validFilePath(oldPath) || !validFilePath(newPath) || path.Dir(oldPath) != path.Dir(newPath) {
		return ErrInvalidPath
	}
	cfg, err := c.currentConfig()
	if err != nil {
		return err
	}
	torrent, err := c.torrent(ctx, cfg, hash)
	if err != nil {
		return err
	}
	return c.renamePath(ctx, cfg, torrent.ID, oldPath, path.Base(newPath))
}

func (c *Client) renamePath(ctx context.Context, cfg config, id int64, oldPath, name string) error {
	arguments := renamePathArguments{IDs: []int64{id}, Path: oldPath, Name: name}
	return c.callMutation(ctx, cfg, "torrent-rename-path", arguments, &renamePathResponse{expectedPath: oldPath, expectedName: name, expectedID: id})
}

func invalidHash(value string) bool { return value == "" || containsControl(value) }

func containsControl(value string) bool { return strings.IndexFunc(value, unicode.IsControl) >= 0 }

func validRenameName(value string) bool {
	return value != "" && strings.TrimSpace(value) != "" && !containsControl(value) && !strings.ContainsAny(value, `/\`) && value != "." && value != ".."
}

func validFilePath(value string) bool {
	if value == "" || containsControl(value) || strings.Contains(value, `\`) || strings.HasPrefix(value, "/") || path.Clean(value) != value {
		return false
	}
	for _, component := range strings.Split(value, "/") {
		if component == "." || component == ".." || component == "" {
			return false
		}
	}
	return true
}
