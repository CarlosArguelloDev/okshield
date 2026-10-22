package docker

import (
	"strconv"
	"strings"
	"time"
)

// Container is a compact view of a container as returned by the list API.
// Only fields OkShield renders are kept, to minimise resident memory.
type Container struct {
	ID       string
	Name     string
	Image    string
	State    string // running, exited, restarting, paused, dead, created
	Status   string // human status, e.g. "Exited (1) 2 minutes ago"
	Created  time.Time
	Ports    []Port
	Networks []NetAttachment
	Mounts   []Mount
}

// ShortID returns the 12-char container id.
func (c Container) ShortID() string {
	if len(c.ID) > 12 {
		return c.ID[:12]
	}
	return c.ID
}

// ExitCode extracts the exit code from Status ("Exited (137) ..."). It returns
// -1 when the container has not exited.
func (c Container) ExitCode() int {
	const prefix = "Exited ("
	if !strings.HasPrefix(c.Status, prefix) {
		return -1
	}
	rest := c.Status[len(prefix):]
	end := strings.IndexByte(rest, ')')
	if end < 0 {
		return -1
	}
	n, err := strconv.Atoi(rest[:end])
	if err != nil {
		return -1
	}
	return n
}

// Port is a published or exposed container port.
type Port struct {
	IP      string
	Private uint16
	Public  uint16
	Proto   string
}

// NetAttachment is a container's endpoint in a network.
type NetAttachment struct {
	Network string
	IP      string
}

// Mount is a volume or bind mount attached to a container.
type Mount struct {
	Type        string // volume, bind, tmpfs
	Name        string
	Source      string
	Destination string
	RW          bool
}

// Network is a Docker network.
type Network struct {
	ID       string
	Name     string
	Driver   string
	Scope    string
	Internal bool
	Subnets  []Subnet
}

// Subnet is an IPAM pool of a network.
type Subnet struct {
	CIDR    string
	Gateway string
}

// Volume is a named Docker volume.
type Volume struct {
	Name       string
	Driver     string
	Mountpoint string
	Scope      string
	CreatedAt  string
}
