//go:build !windows

package docker

import (
	"context"
	"errors"
	"net"
)

func dialPipe(context.Context, string) (net.Conn, error) {
	return nil, errors.New("docker: named pipes are only supported on windows")
}
