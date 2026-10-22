package docker

import (
	"context"
	"net/http"
	"testing"
)

const containersJSON = `[
 {"Id":"bbbbbbbbbbbbbbbbbbbb","Names":["/web"],"Image":"nginx:alpine","State":"running",
  "Status":"Up 3 hours","Created":1700000000,
  "Ports":[{"IP":"0.0.0.0","PrivatePort":80,"PublicPort":8080,"Type":"tcp"}],
  "Mounts":[{"Type":"volume","Name":"webdata","Source":"/var/lib/docker/volumes/webdata/_data","Destination":"/usr/share/nginx/html","RW":true}],
  "NetworkSettings":{"Networks":{"frontend":{"IPAddress":"172.20.0.2"},"bridge":{"IPAddress":"172.17.0.2"}}}},
 {"Id":"aaaaaaaaaaaaaaaaaaaa","Names":["/api"],"Image":"api:1","State":"exited",
  "Status":"Exited (137) 5 minutes ago","Created":1700000100,"Ports":[],"Mounts":[],
  "NetworkSettings":{"Networks":{}}}
]`

func TestListContainers(t *testing.T) {
	c := newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/containers/json" || r.URL.Query().Get("all") != "1" {
			t.Errorf("unexpected request %s", r.URL)
		}
		w.Write([]byte(containersJSON))
	}))
	got, err := c.ListContainers(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 {
		t.Fatalf("len = %d, want 2", len(got))
	}
	api, web := got[0], got[1] // sorted by name
	if api.Name != "api" || web.Name != "web" {
		t.Fatalf("names = %q,%q", api.Name, web.Name)
	}
	if api.ExitCode() != 137 || web.ExitCode() != -1 {
		t.Errorf("exit codes = %d,%d", api.ExitCode(), web.ExitCode())
	}
	if web.ShortID() != "bbbbbbbbbbbb" {
		t.Errorf("ShortID = %q", web.ShortID())
	}
	if len(web.Ports) != 1 || web.Ports[0].Public != 8080 {
		t.Errorf("ports = %+v", web.Ports)
	}
	if len(web.Networks) != 2 || web.Networks[0].Network != "bridge" || web.Networks[1].IP != "172.20.0.2" {
		t.Errorf("networks = %+v", web.Networks)
	}
	if len(web.Mounts) != 1 || web.Mounts[0].Name != "webdata" || !web.Mounts[0].RW {
		t.Errorf("mounts = %+v", web.Mounts)
	}
}

func TestListNetworksAndVolumes(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/networks", func(w http.ResponseWriter, _ *http.Request) {
		w.Write([]byte(`[{"Id":"n2","Name":"host","Driver":"host","Scope":"local","IPAM":{"Config":[]}},
		 {"Id":"n1","Name":"bridge","Driver":"bridge","Scope":"local","IPAM":{"Config":[{"Subnet":"172.17.0.0/16","Gateway":"172.17.0.1"}]}}]`))
	})
	mux.HandleFunc("/volumes", func(w http.ResponseWriter, _ *http.Request) {
		w.Write([]byte(`{"Volumes":[{"Name":"pgdata","Driver":"local","Mountpoint":"/var/lib/docker/volumes/pgdata/_data","Scope":"local"}],"Warnings":null}`))
	})
	c := newTestClient(t, mux)
	ctx := context.Background()

	nets, err := c.ListNetworks(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(nets) != 2 || nets[0].Name != "bridge" || nets[0].Subnets[0].Gateway != "172.17.0.1" || nets[1].Subnets != nil {
		t.Errorf("networks = %+v", nets)
	}

	vols, err := c.ListVolumes(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(vols) != 1 || vols[0].Name != "pgdata" || vols[0].Driver != "local" {
		t.Errorf("volumes = %+v", vols)
	}
}

func TestExitCode(t *testing.T) {
	tests := []struct {
		status string
		want   int
	}{
		{"Exited (0) 1 second ago", 0},
		{"Exited (1) 2 hours ago", 1},
		{"Up 5 minutes", -1},
		{"Restarting (1) 3 seconds ago", -1},
		{"Exited (x)", -1},
		{"Exited (", -1},
	}
	for _, tt := range tests {
		if got := (Container{Status: tt.status}).ExitCode(); got != tt.want {
			t.Errorf("ExitCode(%q) = %d, want %d", tt.status, got, tt.want)
		}
	}
}
