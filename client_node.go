package restconf

import (
	"bytes"
	"context"
	"errors"

	"io"

	"github.com/freeconf/yang/fc"
	"github.com/freeconf/yang/node"
	"github.com/freeconf/yang/nodeutil"
	"github.com/freeconf/yang/val"
)

type clientNode struct {
	support clientSupport
	params  string
	read    node.Node
	edit    node.Node
	found   bool
	method  string
	changes node.Node
	device  string
}

// clientSupport is interface between Device and driver.  Factored out as part of
// testing but also because a lot of what driver does is potentially universal to proxying
// for other protocols and might allow reusablity when other protocols are added
type clientSupport interface {
	clientDo(method string, params string, p *node.Path, payload io.Reader) (node.Node, error)
	clientStream(params string, p *node.Path, ctx context.Context) (<-chan node.Node, error)
}

var noSelection node.Selection

func (self *clientNode) node() node.Node {
	n := &nodeutil.Basic{}
	n.OnBeginEdit = func(r node.NodeRequest) error {
		if !r.EditRoot {
			return nil
		}
		if r.New {
			self.method = "POST"
		} else {
			self.method = "PUT"
		}
		return self.startEditMode(r.Selection.Path)
	}
	n.OnChild = func(r node.ChildRequest) (node.Node, error) {
		if r.IsNavigation() {
			if valid, err := self.validNavigation(r.Target); !valid || err != nil {
				return nil, err
			}
			return n, nil
		}
		if self.edit != nil {
			return self.edit.Child(r)
		}
		if self.read == nil {
			if err := self.startReadMode(r.Selection.Path); err != nil {
				return nil, err
			}
		}
		return self.read.Child(r)
	}
	n.OnDelete = func(r node.NodeRequest) error {
		_, err := self.request("DELETE", r.Selection.Path, noSelection)
		return err
	}
	n.OnNext = func(r node.ListRequest) (node.Node, []val.Value, error) {
		if r.IsNavigation() {
			if valid, err := self.validNavigation(r.Target); !valid || err != nil {
				return nil, nil, err
			}
			return n, r.Key, nil
		}
		if self.edit != nil {
			return self.edit.Next(r)
		}
		if self.read == nil {
			if err := self.startReadMode(r.Selection.Path); err != nil {
				return nil, nil, err
			}
		}
		return self.read.Next(r)
	}
	n.OnField = func(r node.FieldRequest, hnd *node.ValueHandle) error {
		if r.IsNavigation() {
			return nil
		} else if self.edit != nil {
			return self.edit.Field(r, hnd)
		}
		if self.read == nil {
			if err := self.startReadMode(r.Selection.Path); err != nil {
				return err
			}
		}
		return self.read.Field(r, hnd)
	}
	n.OnNotify = func(r node.NotifyRequest) (node.NotifyCloser, error) {
		var params string // TODO: support params
		ctx, cancel := context.WithCancel(context.Background())
		events, err := self.support.clientStream(params, r.Selection.Path, ctx)
		if err != nil {
			cancel()
			return nil, err
		}
		go func() {
			for n := range events {
				r.Send(n)
			}
		}()
		closer := func() error {
			cancel()
			return nil
		}
		return closer, nil
	}
	n.OnAction = func(r node.ActionRequest) (node.Node, error) {
		return self.request("POST", r.Selection.Path, r.Input)
	}
	n.OnEndEdit = func(r node.NodeRequest) error {
		// send request
		if !r.EditRoot {
			return nil
		}
		_, err := self.request(self.method, r.Selection.Path, r.Selection.Split(self.changes))
		return err
	}
	return n
}

func (self *clientNode) startReadMode(path *node.Path) (err error) {
	self.read, err = self.get(path, self.params)
	return
}

func (self *clientNode) startEditMode(path *node.Path) error {
	// add depth = 1 so we can pull first level containers and
	// know what container would be conflicts.  we'll have to pull field
	// values too because there's no url param to exclude those yet.
	params := "depth=1&content=config&with-defaults=trim"
	existing, err := self.get(path, params)
	if err != nil {
		return err
	}
	data := make(map[string]interface{})
	self.changes = nodeutil.ReflectChild(data)
	self.edit = &nodeutil.Extend{
		Base: self.changes,
		OnChild: func(p node.Node, r node.ChildRequest) (node.Node, error) {
			if !r.New && existing != nil {
				return existing.Child(r)
			}
			return p.Child(r)
		},
	}
	return nil
}

func (self *clientNode) validNavigation(target *node.Path) (bool, error) {
	if !self.found {
		_, err := self.request("OPTIONS", target, noSelection)
		if errors.Is(err, fc.NotFoundError) {
			return false, nil
		}
		if err != nil {
			return false, err
		}
		self.found = true
	}
	return true, nil
}

func (self *clientNode) get(p *node.Path, params string) (node.Node, error) {
	return self.support.clientDo("GET", params, p, nil)
}

func (self *clientNode) request(method string, p *node.Path, in node.Selection) (node.Node, error) {
	var payload bytes.Buffer
	if !in.IsNil() {
		js := &nodeutil.JSONWtr{Out: &payload}
		if err := in.InsertInto(js.Node()).LastErr; err != nil {
			return nil, err
		}
	}
	return self.support.clientDo(method, "", p, &payload)
}
