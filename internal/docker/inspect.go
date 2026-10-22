package docker

import (
	"context"
	"net/url"
	"time"
)

// ContainerDetail adds runtime state and resource limits to Container. It is
// the primary input of the anomaly correlator and the decision model.
type ContainerDetail struct {
	Container

	Running      bool
	Restarting   bool
	OOMKilled    bool
	Dead         bool
	ExitCode     int
	Error        string
	StartedAt    time.Time
	FinishedAt   time.Time
	RestartCount int
	Health       string // healthy, unhealthy, starting or "" if no healthcheck

	RestartPolicy string
	MemoryLimit   int64 // bytes, 0 = unlimited
	NanoCPUs      int64 // 1e9 = 1 CPU, 0 = unlimited
	Cmd           []string
	EnvCount      int // number of env vars; values are never kept (secrets)
}

type apiInspect struct {
	ID      string `json:"Id"`
	Name    string `json:"Name"`
	Created string `json:"Created"`
	State   struct {
		Status     string `json:"Status"`
		Running    bool   `json:"Running"`
		Restarting bool   `json:"Restarting"`
		OOMKilled  bool   `json:"OOMKilled"`
		Dead       bool   `json:"Dead"`
		ExitCode   int    `json:"ExitCode"`
		Error      string `json:"Error"`
		StartedAt  string `json:"StartedAt"`
		FinishedAt string `json:"FinishedAt"`
		Health     *struct {
			Status string `json:"Status"`
		} `json:"Health"`
	} `json:"State"`
	RestartCount int `json:"RestartCount"`
	Config       struct {
		Image string   `json:"Image"`
		Env   []string `json:"Env"`
		Cmd   []string `json:"Cmd"`
	} `json:"Config"`
	HostConfig struct {
		Memory        int64 `json:"Memory"`
		NanoCpus      int64 `json:"NanoCpus"`
		RestartPolicy struct {
			Name string `json:"Name"`
		} `json:"RestartPolicy"`
	} `json:"HostConfig"`
	Mounts          []apiMount `json:"Mounts"`
	NetworkSettings struct {
		Ports    map[string][]struct{ HostIP, HostPort string } `json:"Ports"`
		Networks map[string]apiEndpoint                         `json:"Networks"`
	} `json:"NetworkSettings"`
}

// InspectContainer returns detailed state for a container id or name.
func (c *Client) InspectContainer(ctx context.Context, idOrName string) (ContainerDetail, error) {
	var r apiInspect
	if err := c.getJSON(ctx, "/containers/"+url.PathEscape(idOrName)+"/json", &r); err != nil {
		return ContainerDetail{}, err
	}
	d := ContainerDetail{
		Container: Container{
			ID:       r.ID,
			Name:     containerName([]string{r.Name}),
			Image:    r.Config.Image,
			State:    r.State.Status,
			Created:  parseTime(r.Created),
			Mounts:   mapMounts(r.Mounts),
			Networks: mapEndpoints(r.NetworkSettings.Networks),
		},
		Running:       r.State.Running,
		Restarting:    r.State.Restarting,
		OOMKilled:     r.State.OOMKilled,
		Dead:          r.State.Dead,
		ExitCode:      r.State.ExitCode,
		Error:         r.State.Error,
		StartedAt:     parseTime(r.State.StartedAt),
		FinishedAt:    parseTime(r.State.FinishedAt),
		RestartCount:  r.RestartCount,
		RestartPolicy: r.HostConfig.RestartPolicy.Name,
		MemoryLimit:   r.HostConfig.Memory,
		NanoCPUs:      r.HostConfig.NanoCpus,
		Cmd:           r.Config.Cmd,
		EnvCount:      len(r.Config.Env),
	}
	if r.State.Health != nil {
		d.Health = r.State.Health.Status
	}
	return d, nil
}

// parseTime parses Docker RFC3339Nano timestamps; zero values
// ("0001-01-01T00:00:00Z") and errors map to time.Time{}.
func parseTime(s string) time.Time {
	t, err := time.Parse(time.RFC3339Nano, s)
	if err != nil || t.Year() <= 1 {
		return time.Time{}
	}
	return t
}
