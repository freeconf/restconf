package device

import (
	"github.com/freeconf/yang/meta"
	"github.com/freeconf/yang/node"
	"github.com/freeconf/yang/source"
)

// Create device from address string associated with protocol
// often referred to south/east/west bound
type ProtocolHandler func(addr string) (Device, error)

type Device interface {
	SchemaSource() source.Opener
	UiSource() source.Opener
	Browser(module string) (*node.Browser, error)
	Modules() map[string]*meta.Module
	Close()
}
