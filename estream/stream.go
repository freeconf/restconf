package estream

import (
	"time"

	"github.com/freeconf/yang/node"
)

type Stream struct {
	Name                  string
	Description           string
	ReplaySupport         bool
	ReplayLogCreationTime time.Time
	ReplayLogAgedTime     time.Time
	Open                  func() (*node.Selection, error)
}
