package mods

import (
	"errors"
	"fmt"
	"net/url"
	"strconv"
)

type cfHash struct {
	Value string `json:"value"`
	Algo  int    `json:"algo"` // 1 sha1, 2 md5
}

type cfDependency struct {
	ModID        int `json:"modId"`
	RelationType int `json:"relationType"` // 3 = required
}

type cfFile struct {
	ID           int            `json:"id"`
	ModID        int            `json:"modId"`
	FileName     string         `json:"fileName"`
	DownloadURL  *string        `json:"downloadUrl"`
	Hashes       []cfHash       `json:"hashes"`
	GameVersions []string       `json:"gameVersions"`
	Dependencies []cfDependency `json:"dependencies"`
}

func (f *cfFile) sha1() string {
	for _, h := range f.Hashes {
		if h.Algo == 1 {
			return h.Value
		}
	}
	return ""
}

// serverSide is false for a file CurseForge tags as Client-only.
func (f *cfFile) serverSide() bool {
	client, server := false, false
	for _, v := range f.GameVersions {
		switch v {
		case "Client":
			client = true
		case "Server":
			server = true
		}
	}
	return !client || server
}

var cfLoaderTypes = map[string]int{"forge": 1, "fabric": 4, "quilt": 5, "neoforge": 6}

func (c *Client) cfHeaders() (map[string]string, error) {
	if c.CurseForgeKey == "" {
		return nil, errors.New("CurseForge needs DK_CURSEFORGE_API_KEY")
	}
	return map[string]string{"x-api-key": c.CurseForgeKey, "Accept": "application/json"}, nil
}

func (c *Client) cfFile(modID, fileID string) (*cfFile, error) {
	h, err := c.cfHeaders()
	if err != nil {
		return nil, err
	}
	var resp struct {
		Data cfFile `json:"data"`
	}
	u := fmt.Sprintf("%s/v1/mods/%s/files/%s", c.CurseForgeAPI, url.PathEscape(modID), url.PathEscape(fileID))
	if err := c.getJSON(u, h, &resp); err != nil {
		return nil, err
	}
	return &resp.Data, nil
}

func (c *Client) cfLatest(modID string, loaders []string, gameVersion string) (*cfFile, error) {
	h, err := c.cfHeaders()
	if err != nil {
		return nil, err
	}
	q := url.Values{"pageSize": {"50"}}
	if gameVersion != "" {
		q.Set("gameVersion", gameVersion)
	}
	for _, l := range loaders {
		if t, ok := cfLoaderTypes[l]; ok {
			q.Set("modLoaderType", strconv.Itoa(t))
			break
		}
	}
	var resp struct {
		Data []cfFile `json:"data"`
	}
	u := fmt.Sprintf("%s/v1/mods/%s/files?%s", c.CurseForgeAPI, url.PathEscape(modID), q.Encode())
	if err := c.getJSON(u, h, &resp); err != nil {
		return nil, err
	}
	if len(resp.Data) == 0 {
		return nil, fmt.Errorf("curseforge mod %s has no file for loader %v and Minecraft %q", modID, loaders, gameVersion)
	}
	return &resp.Data[0], nil
}

func (c *Client) cfFiles(ids []int) ([]cfFile, error) {
	h, err := c.cfHeaders()
	if err != nil {
		return nil, err
	}
	var resp struct {
		Data []cfFile `json:"data"`
	}
	if err := c.postJSON(c.CurseForgeAPI+"/v1/mods/files", h, map[string][]int{"fileIds": ids}, &resp); err != nil {
		return nil, err
	}
	return resp.Data, nil
}

func cfResolved(f *cfFile) (resolved, error) {
	if f.DownloadURL == nil || *f.DownloadURL == "" {
		return resolved{}, fmt.Errorf("curseforge file %s (mod %d) does not allow third-party downloads; upload it from the Files tab instead", f.FileName, f.ModID)
	}
	sha := f.sha1()
	if sha == "" {
		return resolved{}, fmt.Errorf("curseforge file %s has no sha1", f.FileName)
	}
	return resolved{
		Key:      "curseforge:" + strconv.Itoa(f.ModID),
		Filename: f.FileName,
		URL:      *f.DownloadURL,
		Version:  strconv.Itoa(f.ID),
		Hashes:   Hashes{SHA1: sha},
	}, nil
}

func (c *Client) resolveCurseForge(refs []Ref, loaders []string, gameVersion string) ([]resolved, error) {
	var out []resolved
	seen := map[string]bool{}
	queue := append([]Ref(nil), refs...)
	for len(queue) > 0 {
		if len(out) >= maxResolvedMods {
			return nil, fmt.Errorf("more than %d mods with dependencies", maxResolvedMods)
		}
		ref := queue[0]
		queue = queue[1:]
		if seen[ref.Project] {
			continue
		}
		var (
			f   *cfFile
			err error
		)
		if ref.Version != "" {
			f, err = c.cfFile(ref.Project, ref.Version)
		} else {
			f, err = c.cfLatest(ref.Project, loaders, gameVersion)
		}
		if err != nil {
			return nil, fmt.Errorf("curseforge %s: %w", ref.Project, err)
		}
		seen[ref.Project] = true
		r, err := cfResolved(f)
		if err != nil {
			return nil, err
		}
		out = append(out, r)
		for _, d := range f.Dependencies {
			if d.RelationType == 3 && !seen[strconv.Itoa(d.ModID)] {
				queue = append(queue, Ref{Project: strconv.Itoa(d.ModID)})
			}
		}
	}
	return out, nil
}
