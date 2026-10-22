package docker

import (
	"context"
	"sort"
	"strings"
	"time"
)

// Raw Engine API payloads (subset). Kept private; mapped to compact types.

type apiPort struct {
	IP          string `json:"IP"`
	PrivatePort uint16 `json:"PrivatePort"`
	PublicPort  uint16 `json:"PublicPort"`
	Type        string `json:"Type"`
}

type apiMount struct {
	Type        string `json:"Type"`
	Name        string `json:"Name"`
	Source      string `json:"Source"`
	Destination string `json:"Destination"`
	RW          bool   `json:"RW"`
}

type apiEndpoint struct {
	IPAddress string `json:"IPAddress"`
}

type apiContainer struct {
	ID              string     `json:"Id"`
	Names           []string   `json:"Names"`
	Image           string     `json:"Image"`
	State           string     `json:"State"`
	Status          string     `json:"Status"`
	Created         int64      `json:"Created"`
	Ports           []apiPort  `json:"Ports"`
	Mounts          []apiMount `json:"Mounts"`
	NetworkSettings struct {
		Networks map[string]apiEndpoint `json:"Networks"`
	} `json:"NetworkSettings"`
}

type apiNetwork struct {
	ID       string `json:"Id"`
	Name     string `json:"Name"`
	Driver   string `json:"Driver"`
	Scope    string `json:"Scope"`
	Internal bool   `json:"Internal"`
	IPAM     struct {
		Config []struct {
			Subnet  string `json:"Subnet"`
			Gateway string `json:"Gateway"`
		} `json:"Config"`
	} `json:"IPAM"`
}

type apiVolume struct {
	Name       string `json:"Name"`
	Driver     string `json:"Driver"`
	Mountpoint string `json:"Mountpoint"`
	Scope      string `json:"Scope"`
	CreatedAt  string `json:"CreatedAt"`
}

// ListContainers returns all containers (running and stopped), sorted by name.
func (c *Client) ListContainers(ctx context.Context) ([]Container, error) {
	var raw []apiContainer
	if err := c.getJSON(ctx, "/containers/json?all=1", &raw); err != nil {
		return nil, err
	}
	out := make([]Container, len(raw))
	for i, r := range raw {
		out[i] = Container{
			ID:      r.ID,
			Name:    containerName(r.Names),
			Image:   r.Image,
			State:   r.State,
			Status:  r.Status,
			Created: time.Unix(r.Created, 0),
			Ports:   mapPorts(r.Ports),
			Mounts:  mapMounts(r.Mounts),
		}
		out[i].Networks = mapEndpoints(r.NetworkSettings.Networks)
	}
	sort.Slice(out, func(a, b int) bool { return out[a].Name < out[b].Name })
	return out, nil
}

// ListNetworks returns all networks sorted by name.
func (c *Client) ListNetworks(ctx context.Context) ([]Network, error) {
	var raw []apiNetwork
	if err := c.getJSON(ctx, "/networks", &raw); err != nil {
		return nil, err
	}
	out := make([]Network, len(raw))
	for i, r := range raw {
		n := Network{ID: r.ID, Name: r.Name, Driver: r.Driver, Scope: r.Scope, Internal: r.Internal}
		if len(r.IPAM.Config) > 0 {
			n.Subnets = make([]Subnet, len(r.IPAM.Config))
			for j, cfg := range r.IPAM.Config {
				n.Subnets[j] = Subnet{CIDR: cfg.Subnet, Gateway: cfg.Gateway}
			}
		}
		out[i] = n
	}
	sort.Slice(out, func(a, b int) bool { return out[a].Name < out[b].Name })
	return out, nil
}

// ListVolumes returns all named volumes sorted by name.
func (c *Client) ListVolumes(ctx context.Context) ([]Volume, error) {
	var raw struct {
		Volumes []apiVolume `json:"Volumes"`
	}
	if err := c.getJSON(ctx, "/volumes", &raw); err != nil {
		return nil, err
	}
	out := make([]Volume, len(raw.Volumes))
	for i, r := range raw.Volumes {
		out[i] = Volume(r)
	}
	sort.Slice(out, func(a, b int) bool { return out[a].Name < out[b].Name })
	return out, nil
}

func containerName(names []string) string {
	if len(names) == 0 {
		return ""
	}
	return strings.TrimPrefix(names[0], "/")
}

func mapPorts(in []apiPort) []Port {
	if len(in) == 0 {
		return nil
	}
	out := make([]Port, len(in))
	for i, p := range in {
		out[i] = Port{IP: p.IP, Private: p.PrivatePort, Public: p.PublicPort, Proto: p.Type}
	}
	return out
}

func mapMounts(in []apiMount) []Mount {
	if len(in) == 0 {
		return nil
	}
	out := make([]Mount, len(in))
	for i, m := range in {
		out[i] = Mount(m)
	}
	return out
}

func mapEndpoints(in map[string]apiEndpoint) []NetAttachment {
	if len(in) == 0 {
		return nil
	}
	out := make([]NetAttachment, 0, len(in))
	for name, ep := range in {
		out = append(out, NetAttachment{Network: name, IP: ep.IPAddress})
	}
	sort.Slice(out, func(a, b int) bool { return out[a].Network < out[b].Network })
	return out
}
