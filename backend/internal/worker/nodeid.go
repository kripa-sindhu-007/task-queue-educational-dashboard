package worker

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"os"
)

// NewNodeID returns a cluster-unique node identity of the form
// {hostname}-{shortuuid}. The hostname makes IDs human-readable in the
// dashboard/logs; the random suffix guarantees uniqueness across replicas that
// share a hostname (e.g. multiple containers, or restarts reusing a name).
func NewNodeID() string {
	host, err := os.Hostname()
	if err != nil || host == "" {
		host = "node"
	}
	return fmt.Sprintf("%s-%s", host, shortUUID())
}

func shortUUID() string {
	var b [6]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "000000000000"
	}
	return hex.EncodeToString(b[:])
}

// Hostname returns the OS hostname, or "node" if it cannot be determined.
func Hostname() string {
	host, err := os.Hostname()
	if err != nil || host == "" {
		return "node"
	}
	return host
}
