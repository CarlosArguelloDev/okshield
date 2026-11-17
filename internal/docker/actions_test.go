package docker

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"
)

func TestDockerActions(t *testing.T) {
	var got []string
	var disconnBody struct {
		Container string
		Force     bool
	}
	c := newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		got = append(got, r.Method+" "+r.URL.RequestURI())
		if r.URL.Path == "/networks/n1/disconnect" {
			if err := json.NewDecoder(r.Body).Decode(&disconnBody); err != nil {
				t.Errorf("decode disconnect body: %v", err)
			}
		}
		w.WriteHeader(http.StatusNoContent)
	}))

	ctx := context.Background()
	calls := []struct {
		name string
		fn   func() error
		want string
	}{
		{"start", func() error { return c.StartContainer(ctx, "c1") }, "POST /containers/c1/start"},
		{"stop", func() error { return c.StopContainer(ctx, "c1") }, "POST /containers/c1/stop"},
		{"restart", func() error { return c.RestartContainer(ctx, "c1") }, "POST /containers/c1/restart"},
		{"pause", func() error { return c.PauseContainer(ctx, "c1") }, "POST /containers/c1/pause"},
		{"unpause", func() error { return c.UnpauseContainer(ctx, "c1") }, "POST /containers/c1/unpause"},
		{"rm", func() error { return c.RemoveContainer(ctx, "c1", true) }, "DELETE /containers/c1?force=1"},
		{"disconnect", func() error { return c.DisconnectNetwork(ctx, "n1", "c1") }, "POST /networks/n1/disconnect"},
		{"prune-net", func() error { return c.PruneNetworks(ctx) }, "POST /networks/prune"},
		{"rm-vol", func() error { return c.RemoveVolume(ctx, "v1") }, "DELETE /volumes/v1"},
		{"pull-img", func() error { return c.PullImage(ctx, "nginx:alpine") }, "POST /images/create?fromImage=nginx%3Aalpine"},
		{"rm-img", func() error { return c.RemoveImage(ctx, "img1") }, "DELETE /images/img1"},
	}

	for i, tc := range calls {
		if err := tc.fn(); err != nil {
			t.Fatalf("%s failed: %v", tc.name, err)
		}
		if got[i] != tc.want {
			t.Errorf("%s request = %q, want %q", tc.name, got[i], tc.want)
		}
	}
	if disconnBody.Container != "c1" || !disconnBody.Force {
		t.Errorf("disconnect payload = %+v", disconnBody)
	}
}
