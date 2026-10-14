package mods

import (
	"encoding/json"
	"fmt"
	"net/url"
)

type mrFile struct {
	URL      string `json:"url"`
	Filename string `json:"filename"`
	Primary  bool   `json:"primary"`
	Hashes   struct {
		SHA1   string `json:"sha1"`
		SHA512 string `json:"sha512"`
	} `json:"hashes"`
}

type mrDependency struct {
	ProjectID      string `json:"project_id"`
	VersionID      string `json:"version_id"`
	DependencyType string `json:"dependency_type"`
}

type mrVersion struct {
	ID            string         `json:"id"`
	ProjectID     string         `json:"project_id"`
	VersionNumber string         `json:"version_number"`
	VersionType   string         `json:"version_type"`
	Files         []mrFile       `json:"files"`
	Dependencies  []mrDependency `json:"dependencies"`
	GameVersions  []string       `json:"game_versions"`
	Loaders       []string       `json:"loaders"`
}

func (v *mrVersion) primary() (mrFile, error) {
	for _, f := range v.Files {
		if f.Primary {
			return f, nil
		}
	}
	if len(v.Files) > 0 {
		return v.Files[0], nil
	}
	return mrFile{}, fmt.Errorf("modrinth version %s has no files", v.ID)
}

func (c *Client) mrVersion(id string) (*mrVersion, error) {
	var v mrVersion
	if err := c.getJSON(c.ModrinthAPI+"/v2/version/"+url.PathEscape(id), nil, &v); err != nil {
		return nil, err
	}
	return &v, nil
}

// mrLatest returns the newest version of project matching the filters,
// preferring a release over beta/alpha.
func (c *Client) mrLatest(project string, loaders []string, gameVersion string) (*mrVersion, error) {
	q := url.Values{}
	if len(loaders) > 0 {
		b, _ := json.Marshal(loaders)
		q.Set("loaders", string(b))
	}
	if gameVersion != "" {
		b, _ := json.Marshal([]string{gameVersion})
		q.Set("game_versions", string(b))
	}
	u := c.ModrinthAPI + "/v2/project/" + url.PathEscape(project) + "/version"
	if len(q) > 0 {
		u += "?" + q.Encode()
	}
	var versions []mrVersion
	if err := c.getJSON(u, nil, &versions); err != nil {
		return nil, err
	}
	if len(versions) == 0 {
		return nil, fmt.Errorf("modrinth project %s has no version for loader %v and Minecraft %q", project, loaders, gameVersion)
	}
	for i := range versions {
		if versions[i].VersionType == "release" {
			return &versions[i], nil
		}
	}
	return &versions[0], nil
}

// resolveModrinth turns refs into files, adding required dependencies.
func (c *Client) resolveModrinth(refs []Ref, loaders []string, gameVersion string) ([]resolved, error) {
	var out []resolved
	seen := map[string]bool{}
	queue := append([]Ref(nil), refs...)
	for len(queue) > 0 {
		ref := queue[0]
		queue = queue[1:]
		var (
			v   *mrVersion
			err error
		)
		if ref.Version != "" {
			v, err = c.mrVersion(ref.Version)
		} else {
			v, err = c.mrLatest(ref.Project, loaders, gameVersion)
		}
		if err != nil {
			return nil, fmt.Errorf("modrinth %s: %w", ref.Project, err)
		}
		if seen[v.ProjectID] {
			continue
		}
		seen[v.ProjectID] = true
		// A ref may name the project by slug; the lock is keyed by id.
		seen[ref.Project] = true
		f, err := v.primary()
		if err != nil {
			return nil, err
		}
		out = append(out, resolved{
			Key:      "modrinth:" + v.ProjectID,
			Filename: f.Filename,
			URL:      f.URL,
			Version:  v.ID,
			Hashes:   Hashes{SHA1: f.Hashes.SHA1, SHA512: f.Hashes.SHA512},
		})
		for _, d := range v.Dependencies {
			if d.DependencyType != "required" || (d.ProjectID != "" && seen[d.ProjectID]) {
				continue
			}
			if d.ProjectID == "" && d.VersionID == "" {
				continue
			}
			queue = append(queue, Ref{Project: d.ProjectID, Version: d.VersionID})
		}
	}
	return out, nil
}
