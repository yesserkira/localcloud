package id

import (
	"crypto/rand"
	"time"

	"github.com/oklog/ulid/v2"
)

// New generates a new ULID string.
func New() string {
	return ulid.MustNew(ulid.Timestamp(time.Now()), rand.Reader).String()
}

// NewWithPrefix generates a ULID with a descriptive prefix.
func NewWithPrefix(prefix string) string {
	return prefix + "_" + New()
}

// Event returns a new event ID.
func Event() string { return NewWithPrefix("evt") }

// Run returns a new run ID.
func Run() string { return NewWithPrefix("run") }

// Scenario returns a new scenario ID.
func Scenario() string { return NewWithPrefix("scn") }

// ReplayRun returns a new replay run ID.
func ReplayRun() string { return NewWithPrefix("rpr") }

// FaultRule returns a new fault rule ID.
func FaultRule() string { return NewWithPrefix("flt") }

// ConfigSnapshot returns a new config snapshot ID.
func ConfigSnapshot() string { return NewWithPrefix("cfg") }

// Correlation returns a new correlation ID.
func Correlation() string { return NewWithPrefix("cor") }
