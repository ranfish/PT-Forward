package qbittorrent

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"github.com/ranfish/pt-forward/internal/model"
	"go.uber.org/zap"
)

func (c *QBClient) GetTorrentByHash(ctx context.Context, hash string) (*model.TorrentInfo, error) {
	resp, err := c.get(ctx, "/api/v2/torrents/info?hashes="+url.QueryEscape(hash))
	if err != nil {
		return nil, c.wrapErr(11002, "get torrent by hash", err)
	}
	defer func() { _ = resp.Body.Close() }()

	var torrents []qbTorrent
	if err := json.NewDecoder(resp.Body).Decode(&torrents); err != nil {
		return nil, c.wrapErr(11002, "decode torrents", err)
	}
	if len(torrents) == 0 {
		return nil, nil
	}
	info := torrents[0].toModel()
	return info, nil
}

func (c *QBClient) GetSeedingTorrents(ctx context.Context) ([]*model.TorrentInfo, error) {
	resp, err := c.get(ctx, "/api/v2/torrents/info?filter=completed")
	if err != nil {
		return nil, c.wrapErr(11002, "get seeding torrents", err)
	}
	defer func() { _ = resp.Body.Close() }()

	var torrents []qbTorrent
	if err := json.NewDecoder(resp.Body).Decode(&torrents); err != nil {
		return nil, c.wrapErr(11002, "decode seeding torrents", err)
	}

	result := make([]*model.TorrentInfo, 0, len(torrents))
	for i := range torrents {
		result = append(result, torrents[i].toModel())
	}
	return result, nil
}

func (c *QBClient) GetTorrentsByPath(ctx context.Context, savePath string) ([]*model.TorrentInfo, error) {
	resp, err := c.get(ctx, "/api/v2/torrents/info")
	if err != nil {
		return nil, c.wrapErr(11002, "get torrents by path", err)
	}
	defer func() { _ = resp.Body.Close() }()

	var torrents []qbTorrent
	if err := json.NewDecoder(resp.Body).Decode(&torrents); err != nil {
		return nil, c.wrapErr(11002, "decode torrents", err)
	}

	result := make([]*model.TorrentInfo, 0)
	for i := range torrents {
		if torrents[i].SavePath == savePath {
			result = append(result, torrents[i].toModel())
		}
	}
	return result, nil
}

func (c *QBClient) GetMainData(ctx context.Context) (*model.Maindata, error) {
	resp, err := c.get(ctx, "/api/v2/sync/maindata?rid=0")
	if err != nil {
		return nil, c.wrapErr(11002, "get maindata", err)
	}
	defer func() { _ = resp.Body.Close() }()

	var raw struct {
		Torrents    map[string]qbTorrent `json:"torrents"`
		ServerState struct {
			FreeSpaceOnDisk int64 `json:"free_space_on_disk"`
			UpInfoSpeed     int64 `json:"up_info_speed"`
		} `json:"server_state"`
		Rid        int               `json:"rid"`
		Categories map[string]string `json:"categories"`
		Tags       string            `json:"tags"`
		FullUpdate bool              `json:"full_update"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&raw); err != nil {
		return nil, c.wrapErr(11002, "decode maindata", err)
	}

	torrents := make(map[string]model.TorrentInfo, len(raw.Torrents))
	for hash, t := range raw.Torrents {
		torrents[hash] = *t.toModel()
	}

	var tags []string
	if raw.Tags != "" {
		tags = strings.Split(raw.Tags, ", ")
	}

	return &model.Maindata{
		Torrents:    torrents,
		FreeSpace:   raw.ServerState.FreeSpaceOnDisk,
		CategoryMap: raw.Categories,
		Tags:        tags,
		ServerState: model.ServerState{UploadSpeed: raw.ServerState.UpInfoSpeed},
	}, nil
}

func (c *QBClient) GetMainDataIncremental(ctx context.Context, rid int) (*model.Maindata, int, error) {
	path := fmt.Sprintf("/api/v2/sync/maindata?rid=%d", rid)
	resp, err := c.get(ctx, path)
	if err != nil {
		return nil, 0, c.wrapErr(11002, "get maindata incremental", err)
	}
	defer func() { _ = resp.Body.Close() }()

	var raw struct {
		Torrents        map[string]qbTorrent `json:"torrents"`
		TorrentsRemoved []string             `json:"torrents_removed"`
		ServerState     struct {
			FreeSpaceOnDisk int64 `json:"free_space_on_disk"`
		} `json:"server_state"`
		Rid        int               `json:"rid"`
		Categories map[string]string `json:"categories"`
		Tags       string            `json:"tags"`
		FullUpdate bool              `json:"full_update"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&raw); err != nil {
		return nil, 0, c.wrapErr(11002, "decode maindata incremental", err)
	}

	torrents := make(map[string]model.TorrentInfo, len(raw.Torrents))
	for hash, t := range raw.Torrents {
		torrents[hash] = *t.toModel()
	}

	var tags []string
	if raw.Tags != "" {
		tags = strings.Split(raw.Tags, ", ")
	}

	return &model.Maindata{
		Torrents:    torrents,
		FreeSpace:   raw.ServerState.FreeSpaceOnDisk,
		CategoryMap: raw.Categories,
		Tags:        tags,
	}, raw.Rid, nil
}

func (c *QBClient) AddFromFile(ctx context.Context, data []byte, opts model.AddTorrentOptions) (*model.AddResult, error) {
	infoHash, err := computeInfoHash(data)
	if err != nil {
		c.logger.Debug("compute info hash failed, will query after add", zap.Error(err))
	}

	body, contentType, err := buildMultipartBody(data, opts)
	if err != nil {
		return nil, c.wrapErr(11007, "build multipart body", err)
	}

	bodyBytes, err := io.ReadAll(body)
	if err != nil {
		return nil, c.wrapErr(11007, "read multipart body", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/api/v2/torrents/add", bytes.NewReader(bodyBytes))
	if err != nil {
		return nil, c.wrapErr(11007, "build add request", err)
	}
	req.Header.Set("Content-Type", contentType)
	req.Header.Set("Referer", c.baseURL)

	resp, err := c.client.Do(req)
	if err != nil {
		return nil, c.wrapErr(11002, "add torrent request", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode == http.StatusForbidden {
		c.logger.Debug("session expired during add, re-login", zap.String("client", c.cfg.Name))
		if loginErr := c.login(ctx); loginErr != nil {
			return nil, c.wrapErr(11002, "re-login for add torrent", loginErr)
		}
		req2, _ := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/api/v2/torrents/add", bytes.NewReader(bodyBytes))
		req2.Header.Set("Content-Type", contentType)
		req2.Header.Set("Referer", c.baseURL)
		resp2, err := c.client.Do(req2)
		if err != nil {
			return nil, c.wrapErr(11002, "add torrent retry request", err)
		}
		defer func() { _ = resp2.Body.Close() }()
		resp = resp2
	}

	if resp.StatusCode == http.StatusUnsupportedMediaType {
		return nil, c.newErr(11007, "invalid torrent file")
	}
	if resp.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(resp.Body)
		return nil, c.newErr(11007, fmt.Sprintf("add torrent returned %d: %s", resp.StatusCode, string(b)))
	}

	if infoHash == "" {
		infoHash, err = c.queryHashAfterAdd(ctx, data)
		if err != nil {
			return nil, err
		}
	}

	name := ""
	if t, _ := c.GetTorrentByHash(ctx, infoHash); t != nil {
		name = t.Name
	}

	return &model.AddResult{
		InfoHash:    infoHash,
		Name:        name,
		IsDuplicate: false,
	}, nil
}

func (c *QBClient) queryHashAfterAdd(ctx context.Context, data []byte) (string, error) {
	if hash, err := computeInfoHash(data); err == nil {
		return hash, nil
	}

	resp, err := c.get(ctx, "/api/v2/torrents/info?sort=added_on&reverse=true&limit=1")
	if err != nil {
		return "", c.wrapErr(11007, "query recent torrent", err)
	}
	defer func() { _ = resp.Body.Close() }()

	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		return "", c.newErr(11007, fmt.Sprintf("query recent torrent returned %d", resp.StatusCode))
	}

	var torrents []struct {
		Hash string `json:"hash"`
	}
	if err := json.Unmarshal(body, &torrents); err != nil || len(torrents) == 0 {
		return "", c.newErr(11007, "no torrent found after add")
	}

	return strings.ToLower(torrents[0].Hash), nil
}

func (c *QBClient) ExportTorrent(ctx context.Context, hash string) ([]byte, error) {
	resp, err := c.get(ctx, "/api/v2/torrents/export?hash="+url.QueryEscape(hash))
	if err != nil {
		return nil, c.wrapErr(11002, "export torrent", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return nil, c.newErr(11005, fmt.Sprintf("export torrent returned %d", resp.StatusCode))
	}

	b, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, c.wrapErr(11002, "read export response", err)
	}
	return b, nil
}

func (c *QBClient) DeleteTorrent(ctx context.Context, hash string, deleteFiles bool) error {
	data := url.Values{
		"hashes":      {hash},
		"deleteFiles": {strconv.FormatBool(deleteFiles)},
	}
	resp, err := c.postForm(ctx, "/api/v2/torrents/delete", data)
	if err != nil {
		return c.wrapErr(11002, "delete torrent", err)
	}
	defer func() { _ = resp.Body.Close() }()
	return nil
}

func (c *QBClient) BatchDeleteTorrents(ctx context.Context, hashes []string, deleteFiles bool) error {
	data := url.Values{
		"hashes":      {strings.Join(hashes, "|")},
		"deleteFiles": {strconv.FormatBool(deleteFiles)},
	}
	resp, err := c.postForm(ctx, "/api/v2/torrents/delete", data)
	if err != nil {
		return c.wrapErr(11002, "batch delete torrents", err)
	}
	defer func() { _ = resp.Body.Close() }()
	return nil
}

func (c *QBClient) PauseTorrent(ctx context.Context, hash string) error {
	data := url.Values{"hashes": {hash}}
	resp, err := c.postForm(ctx, c.pauseEndpoint(), data)
	if err != nil {
		return c.wrapErr(11002, "pause torrent", err)
	}
	defer func() { _ = resp.Body.Close() }()
	return nil
}

func (c *QBClient) ResumeTorrent(ctx context.Context, hash string) error {
	data := url.Values{"hashes": {hash}}
	resp, err := c.postForm(ctx, c.resumeEndpoint(), data)
	if err != nil {
		return c.wrapErr(11002, "resume torrent", err)
	}
	defer func() { _ = resp.Body.Close() }()
	return nil
}

func (c *QBClient) Reannounce(ctx context.Context, hash string) error {
	data := url.Values{"hashes": {hash}}
	resp, err := c.postForm(ctx, "/api/v2/torrents/reannounce", data)
	if err != nil {
		return c.wrapErr(11002, "reannounce", err)
	}
	defer func() { _ = resp.Body.Close() }()
	return nil
}

func (c *QBClient) Recheck(ctx context.Context, hash string) error {
	data := url.Values{"hashes": {hash}}
	resp, err := c.postForm(ctx, "/api/v2/torrents/recheck", data)
	if err != nil {
		return c.wrapErr(11002, "recheck", err)
	}
	defer func() { _ = resp.Body.Close() }()
	return nil
}

func (c *QBClient) SetTorrentTags(ctx context.Context, hash string, tags []string) error {
	current, err := c.GetTorrentByHash(ctx, hash)
	if err != nil || current == nil {
		return err
	}
	if len(current.Tags) > 0 {
		removeData := url.Values{
			"hashes": {hash},
			"tags":   {strings.Join(current.Tags, ",")},
		}
		resp, err := c.postForm(ctx, "/api/v2/torrents/removeTags", removeData)
		if err != nil {
			return c.wrapErr(11002, "remove old tags", err)
		}
		_ = resp.Body.Close()
	}
	if len(tags) > 0 {
		addData := url.Values{
			"hashes": {hash},
			"tags":   {strings.Join(tags, ",")},
		}
		resp, err := c.postForm(ctx, "/api/v2/torrents/addTags", addData)
		if err != nil {
			return c.wrapErr(11002, "add tags", err)
		}
		_ = resp.Body.Close()
	}
	return nil
}

func (c *QBClient) RemoveTorrentTags(ctx context.Context, hash string, tags []string) error {
	data := url.Values{
		"hashes": {hash},
		"tags":   {strings.Join(tags, ",")},
	}
	resp, err := c.postForm(ctx, "/api/v2/torrents/removeTags", data)
	if err != nil {
		return c.wrapErr(11002, "remove tags", err)
	}
	defer func() { _ = resp.Body.Close() }()
	return nil
}

func (c *QBClient) SetCategory(ctx context.Context, hash string, category string) error {
	data := url.Values{
		"hashes":   {hash},
		"category": {category},
	}
	resp, err := c.postForm(ctx, "/api/v2/torrents/setCategory", data)
	if err != nil {
		return c.wrapErr(11002, "set category", err)
	}
	defer func() { _ = resp.Body.Close() }()
	return nil
}

func (c *QBClient) SetSavePath(ctx context.Context, hash string, savePath string) error {
	data := url.Values{
		"hashes":   {hash},
		"location": {savePath},
	}
	resp, err := c.postForm(ctx, "/api/v2/torrents/setLocation", data)
	if err != nil {
		return c.wrapErr(11002, "set save path", err)
	}
	defer func() { _ = resp.Body.Close() }()
	return nil
}

func (c *QBClient) SetSuperSeeding(ctx context.Context, hash string, enable bool) error {
	data := url.Values{
		"hashes": {hash},
		"value":  {strconv.FormatBool(enable)},
	}
	resp, err := c.postForm(ctx, "/api/v2/torrents/setSuperSeeding", data)
	if err != nil {
		return c.wrapErr(11002, "set super seeding", err)
	}
	defer func() { _ = resp.Body.Close() }()
	return nil
}

func (c *QBClient) SetUploadLimit(ctx context.Context, infoHash string, limitBytesPerSec int64) error {
	data := url.Values{
		"hashes": {infoHash},
		"limit":  {strconv.FormatInt(limitBytesPerSec, 10)},
	}
	resp, err := c.postForm(ctx, "/api/v2/torrents/setUploadLimit", data)
	if err != nil {
		return c.wrapErr(11002, "set upload limit", err)
	}
	defer func() { _ = resp.Body.Close() }()
	return nil
}

func (c *QBClient) PauseAllDownloads(ctx context.Context) error {
	data := url.Values{"hashes": {"all"}}
	resp, err := c.postForm(ctx, c.pauseEndpoint(), data)
	if err != nil {
		return c.wrapErr(11002, "pause all", err)
	}
	defer func() { _ = resp.Body.Close() }()
	return nil
}

func (c *QBClient) ResumeAllDownloads(ctx context.Context) error {
	data := url.Values{"hashes": {"all"}}
	resp, err := c.postForm(ctx, c.resumeEndpoint(), data)
	if err != nil {
		return c.wrapErr(11002, "resume all", err)
	}
	defer func() { _ = resp.Body.Close() }()
	return nil
}

func (c *QBClient) GetFreeSpace(ctx context.Context) (int64, error) {
	md, err := c.GetMainData(ctx)
	if err != nil {
		return 0, err
	}
	return md.FreeSpace, nil
}

func (c *QBClient) CheckExists(ctx context.Context, infoHash string) (bool, error) {
	t, err := c.GetTorrentByHash(ctx, infoHash)
	if err != nil {
		return false, err
	}
	return t != nil, nil
}
