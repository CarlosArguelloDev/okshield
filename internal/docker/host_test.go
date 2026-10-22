package docker

import "testing"

func TestParseHost(t *testing.T) {
	tests := []struct {
		in      string
		want    Endpoint
		wantErr bool
	}{
		{"unix:///var/run/docker.sock", Endpoint{"unix", "/var/run/docker.sock"}, false},
		{"tcp://10.0.0.5:2376", Endpoint{"tcp", "10.0.0.5:2376"}, false},
		{"tcp://docker", Endpoint{"tcp", "docker:2375"}, false},
		{"npipe:////./pipe/docker_engine", Endpoint{"npipe", `\\.\pipe\docker_engine`}, false},
		{"unix://", Endpoint{}, true},
		{"ssh://pi", Endpoint{}, true},
		{"/var/run/docker.sock", Endpoint{}, true},
	}
	for _, tt := range tests {
		t.Run(tt.in, func(t *testing.T) {
			got, err := ParseHost(tt.in)
			if (err != nil) != tt.wantErr {
				t.Fatalf("err = %v, wantErr %v", err, tt.wantErr)
			}
			if got != tt.want {
				t.Errorf("got %+v, want %+v", got, tt.want)
			}
		})
	}
}

func TestParseHostEmptyUsesDefault(t *testing.T) {
	got, err := ParseHost("")
	if err != nil {
		t.Fatal(err)
	}
	want, _ := ParseHost(DefaultHost())
	if got != want {
		t.Errorf("got %+v, want default %+v", got, want)
	}
}
