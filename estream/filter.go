package estream

import "github.com/freeconf/yang/node"

type Filter struct {
	Name   string
	Filter func(*node.Selection) *node.Selection
}

func (f Filter) Empty() bool {
	return f.Name == "" && f.Filter == nil
}
