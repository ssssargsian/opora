package document

import (
	"bufio"
	"bytes"
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"net"
	"strings"
	"time"
)

var ErrMalwareDetected = errors.New("malware detected")

type Scanner interface {
	Scan(context.Context, []byte) error
}
type ClamAVScanner struct {
	address string
	dialer  net.Dialer
}

func NewClamAVScanner(address string) *ClamAVScanner {
	return &ClamAVScanner{address: address, dialer: net.Dialer{Timeout: 5 * time.Second}}
}
func (c *ClamAVScanner) Scan(ctx context.Context, data []byte) error {
	conn, err := c.dialer.DialContext(ctx, "tcp", c.address)
	if err != nil {
		return fmt.Errorf("clamav unavailable: %w", err)
	}
	defer func() { _ = conn.Close() }()
	_ = conn.SetDeadline(time.Now().Add(30 * time.Second))
	if _, err = conn.Write([]byte("zINSTREAM\x00")); err != nil {
		return err
	}
	for offset := 0; offset < len(data); {
		end := offset + 32*1024
		if end > len(data) {
			end = len(data)
		}
		var length [4]byte
		chunkLength := uint32(end - offset) // #nosec G115 -- each chunk is bounded to 32 KiB.
		binary.BigEndian.PutUint32(length[:], chunkLength)
		if _, err = conn.Write(length[:]); err != nil {
			return err
		}
		if _, err = conn.Write(data[offset:end]); err != nil {
			return err
		}
		offset = end
	}
	if _, err = conn.Write([]byte{0, 0, 0, 0}); err != nil {
		return err
	}
	response, err := bufio.NewReader(conn).ReadString(0)
	if err != nil {
		return err
	}
	response = strings.TrimSpace(strings.TrimSuffix(response, "\x00"))
	if strings.HasSuffix(response, "OK") {
		return nil
	}
	if strings.Contains(response, "FOUND") {
		return ErrMalwareDetected
	}
	return fmt.Errorf("clamav scan failed: %s", sanitizeScanResponse(response))
}
func sanitizeScanResponse(value string) string {
	value = string(bytes.Map(func(r rune) rune {
		if r < ' ' || r > 126 {
			return -1
		}
		return r
	}, []byte(value)))
	if len(value) > 120 {
		return value[:120]
	}
	return value
}
