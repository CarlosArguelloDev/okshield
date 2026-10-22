package docker

import (
	"context"
	"net/http"
	"testing"
)

const inspectJSON = `{
 "Id":"cccccccccccccccccccccccc","Name":"/worker","Created":"2026-10-20T10:00:00.123456789Z",
 "State":{"Status":"exited","Running":false,"Restarting":false,"OOMKilled":true,"Dead":false,
  "ExitCode":137,"Error":"","StartedAt":"2026-10-22T09:00:00Z","FinishedAt":"2026-10-22T09:05:00Z",
  "Health":{"Status":"unhealthy"}},
 "RestartCount":4,
 "Config":{"Image":"worker:2","Env":["PATH=/usr/bin","DB_PASSWORD=secret"],"Cmd":["./worker","-v"]},
 "HostConfig":{"Memory":67108864,"NanoCpus":500000000,"RestartPolicy":{"Name":"on-failure"}},
 "Mounts":[{"Type":"bind","Source":"/srv/data","Destination":"/data","RW":false}],
 "NetworkSettings":{"Ports":{},"Networks":{"backend":{"IPAddress":"172.21.0.4"}}}
}`

func TestInspectContainer(t *testing.T) {
	c := newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/containers/worker/json" {
			t.Errorf("unexpected path %s", r.URL.Path)
		}
		w.Write([]byte(inspectJSON))
	}))
	d, err := c.InspectContainer(context.Background(), "worker")
	if err != nil {
		t.Fatal(err)
	}
	checks := []struct {
		name string
		ok   bool
	}{
		{"name", d.Name == "worker"},
		{"oom", d.OOMKilled},
		{"exit", d.ExitCode == 137},
		{"restarts", d.RestartCount == 4},
		{"health", d.Health == "unhealthy"},
		{"policy", d.RestartPolicy == "on-failure"},
		{"memlimit", d.MemoryLimit == 64<<20},
		{"cpus", d.NanoCPUs == 5e8},
		{"envcount", d.EnvCount == 2},
		{"started", !d.StartedAt.IsZero()},
		{"mount ro", len(d.Mounts) == 1 && !d.Mounts[0].RW},
		{"network", len(d.Networks) == 1 && d.Networks[0].IP == "172.21.0.4"},
	}
	for _, c := range checks {
		if !c.ok {
			t.Errorf("check %q failed: %+v", c.name, d)
		}
	}
}

func TestParseTimeZero(t *testing.T) {
	if !parseTime("0001-01-01T00:00:00Z").IsZero() || !parseTime("garbage").IsZero() {
		t.Error("zero/invalid timestamps should map to time.Time{}")
	}
}
