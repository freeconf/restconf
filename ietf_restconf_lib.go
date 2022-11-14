package restconf

import (
	"github.com/freeconf/yang/node"
	"github.com/freeconf/yang/nodeutil"
)

func IetfRestconfLib() node.Node {
	return &nodeutil.Basic{}
}
