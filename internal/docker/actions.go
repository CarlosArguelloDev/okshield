package docker

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/url"
)

// StartContainer starts a stopped or created container.
func (c *Client) StartContainer(ctx context.Context, id string) error {
	return c.doNoContent(ctx, http.MethodPost, "/containers/"+url.PathEscape(id)+"/start", nil)
}

// StopContainer gracefully stops a running or paused container.
func (c *Client) StopContainer(ctx context.Context, id string) error {
	return c.doNoContent(ctx, http.MethodPost, "/containers/"+url.PathEscape(id)+"/stop", nil)
}

// RestartContainer restarts a container.
func (c *Client) RestartContainer(ctx context.Context, id string) error {
	return c.doNoContent(ctx, http.MethodPost, "/containers/"+url.PathEscape(id)+"/restart", nil)
}

// PauseContainer pauses processes within a running container.
func (c *Client) PauseContainer(ctx context.Context, id string) error {
	return c.doNoContent(ctx, http.MethodPost, "/containers/"+url.PathEscape(id)+"/pause", nil)
}

// UnpauseContainer resumes a paused container.
func (c *Client) UnpauseContainer(ctx context.Context, id string) error {
	return c.doNoContent(ctx, http.MethodPost, "/containers/"+url.PathEscape(id)+"/unpause", nil)
}

// RemoveContainer deletes a container (optionally forcing removal of running ones).
func (c *Client) RemoveContainer(ctx context.Context, id string, force bool) error {
	q := ""
	if force {
		q = "?force=1"
	}
	return c.doNoContent(ctx, http.MethodDelete, "/containers/"+url.PathEscape(id)+q, nil)
}

// DisconnectNetwork disconnects a container from a network.
func (c *Client) DisconnectNetwork(ctx context.Context, netID, containerID string) error {
	payload, err := json.Marshal(struct {
		Container string
		Force     bool
	}{Container: containerID, Force: true})
	if err != nil {
		return err
	}
	return c.doNoContent(ctx, http.MethodPost, "/networks/"+url.PathEscape(netID)+"/disconnect", bytes.NewReader(payload))
}

// PruneNetworks deletes all unused networks.
func (c *Client) PruneNetworks(ctx context.Context) error {
	return c.doNoContent(ctx, http.MethodPost, "/networks/prune", nil)
}

// RemoveVolume removes a named volume.
func (c *Client) RemoveVolume(ctx context.Context, name string) error {
	return c.doNoContent(ctx, http.MethodDelete, "/volumes/"+url.PathEscape(name), nil)
}

// PullImage pulls an image reference from a registry.
func (c *Client) PullImage(ctx context.Context, ref string) error {
	return c.doNoContent(ctx, http.MethodPost, "/images/create?fromImage="+url.QueryEscape(ref), nil)
}

// RemoveImage deletes an image by ID or tag.
func (c *Client) RemoveImage(ctx context.Context, id string) error {
	return c.doNoContent(ctx, http.MethodDelete, "/images/"+url.PathEscape(id), nil)
}
