package union

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
)

// QueryBlacklist searches the Union Hub blacklist for an entry matching
// username (matched by email or id).
func (c *Client) QueryBlacklist(ctx context.Context, username string) (*BlacklistEntry, error) {
	path := "/blacklist/query?q=" + url.QueryEscape(username)
	body, err := c.request(ctx, http.MethodGet, path, nil, nil)
	if err != nil {
		return nil, fmt.Errorf("query blacklist: %w", err)
	}

	entry, err := findBlacklistEntry(body, username)
	if err != nil {
		return nil, err
	}
	if entry == nil {
		return nil, ErrBlacklistEntryNotFound
	}
	return entry, nil
}

// CreateBlacklist submits a new blacklist entry to the Union Hub.
func (c *Client) CreateBlacklist(ctx context.Context, entry BlacklistEntry) error {
	_, err := c.request(ctx, http.MethodPost, "/blacklist/restful", map[string]string{
		"email":  entry.Email,
		"reason": entry.Reason,
	}, nil)
	if err != nil {
		return fmt.Errorf("create blacklist: %w", err)
	}
	return nil
}

// DeleteBlacklist removes a blacklist entry from the Union Hub by resolving
// username to an entry id and calling the Hub delete endpoint.
func (c *Client) DeleteBlacklist(ctx context.Context, username string) error {
	entry, err := c.QueryBlacklist(ctx, username)
	if err != nil {
		return fmt.Errorf("resolve blacklist entry: %w", err)
	}
	if entry.ID == "" {
		return ErrBlacklistEntryNotFound
	}

	path := "/blacklist/restful/" + url.PathEscape(entry.ID)
	if _, err := c.request(ctx, http.MethodDelete, path, nil, nil); err != nil {
		return fmt.Errorf("delete blacklist: %w", err)
	}
	return nil
}

func findBlacklistEntry(body []byte, username string) (*BlacklistEntry, error) {
	var err error

	var list []BlacklistEntry
	if err = json.Unmarshal(body, &list); err == nil {
		return matchEntry(list, username), nil
	}

	var wrapped struct {
		Items []BlacklistEntry `json:"items"`
		Data  []BlacklistEntry `json:"data"`
	}
	if err = json.Unmarshal(body, &wrapped); err == nil {
		if len(wrapped.Items) > 0 {
			return matchEntry(wrapped.Items, username), nil
		}
		if len(wrapped.Data) > 0 {
			return matchEntry(wrapped.Data, username), nil
		}
		return nil, nil
	}

	var single BlacklistEntry
	if err = json.Unmarshal(body, &single); err == nil {
		return &single, nil
	}

	return nil, fmt.Errorf("decode blacklist response: %w", err)
}

func matchEntry(entries []BlacklistEntry, username string) *BlacklistEntry {
	for i := range entries {
		if entries[i].Email == username || entries[i].ID == username {
			return &entries[i]
		}
	}
	return nil
}
