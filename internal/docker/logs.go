package docker

import (
	"bufio"
	"bytes"
	"context"
	"encoding/binary"
	"fmt"
	"io"
	"net/url"
	"strconv"
	"strings"
	"sync"
)

// MaxLogLines is the hard upper bound of log lines fetched per container
// request to honour the low-footprint rule (AGENTS.md §5).
const MaxLogLines = 500

var bufPool = sync.Pool{
	New: func() any {
		b := make([]byte, 0, 4096)
		return &b
	},
}

// ContainerLogs returns up to `tail` recent log lines (capped at MaxLogLines)
// from a container, transparently demultiplexing Docker's 8-byte stream frames
// or reading raw TTY output.
func (c *Client) ContainerLogs(ctx context.Context, idOrName string, tail int) ([]string, error) {
	if tail <= 0 || tail > MaxLogLines {
		tail = MaxLogLines
	}
	path := fmt.Sprintf("/containers/%s/logs?stdout=1&stderr=1&tail=%s",
		url.PathEscape(idOrName), strconv.Itoa(tail))
	body, err := c.do(ctx, path)
	if err != nil {
		return nil, err
	}
	defer body.Close()

	// Cap total read to 512 KiB so a pathological single line can never blow RAM.
	limited := io.LimitReader(body, 512*1024)
	return demuxLines(limited, tail)
}

func demuxLines(r io.Reader, maxLines int) ([]string, error) {
	br := bufio.NewReaderSize(r, 8192)
	peek, err := br.Peek(8)
	if err != nil && err != io.EOF && err != bufio.ErrBufferFull {
		if len(peek) == 0 {
			return nil, nil
		}
	}

	// Docker multiplexed stream frame header: [0|1|2, 0, 0, 0, size_be32]
	isMux := len(peek) == 8 && (peek[0] <= 2) && peek[1] == 0 && peek[2] == 0 && peek[3] == 0

	var plain bytes.Buffer
	if isMux {
		var hdr [8]byte
		bufPtr := bufPool.Get().(*[]byte)
		buf := *bufPtr
		defer func() {
			*bufPtr = buf[:0]
			bufPool.Put(bufPtr)
		}()

		for {
			_, err := io.ReadFull(br, hdr[:])
			if err == io.EOF || err == io.ErrUnexpectedEOF {
				break
			}
			if err != nil {
				return nil, err
			}
			sz := int(binary.BigEndian.Uint32(hdr[4:8]))
			if sz <= 0 {
				continue
			}
			if sz > 256*1024 {
				sz = 256 * 1024
			}
			if cap(buf) < sz {
				buf = make([]byte, sz)
			} else {
				buf = buf[:sz]
			}
			n, err := io.ReadFull(br, buf)
			plain.Write(buf[:n])
			if err != nil {
				break
			}
		}
	} else {
		_, _ = io.Copy(&plain, br)
	}

	raw := strings.Split(strings.ReplaceAll(plain.String(), "\r\n", "\n"), "\n")
	lines := make([]string, 0, min(len(raw), maxLines))
	for _, line := range raw {
		line = strings.TrimRight(line, "\r")
		if line == "" {
			continue
		}
		lines = append(lines, line)
	}
	if len(lines) > maxLines {
		lines = lines[len(lines)-maxLines:]
	}
	return lines, nil
}
