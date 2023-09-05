package client

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"reflect"

	"io"

	"github.com/freeconf/restconf"
	"github.com/freeconf/yang/fc"
	"github.com/freeconf/yang/meta"
	"github.com/freeconf/yang/node"
	"github.com/freeconf/yang/nodeutil"
	"github.com/freeconf/yang/val"
)

type clientNode struct {
	support    clientSupport
	params     string
	read       node.Node
	edit       node.Node
	method     string
	changes    node.Node
	device     string
	compliance restconf.ComplianceOptions
}

// clientSupport is interface between Device and driver.  Factored out as part of
// testing but also because a lot of what driver does is potentially universal to proxying
// for other protocols and might allow reusablity when other protocols are added
type clientSupport interface {
	clientDo(method string, params string, p *node.Path, payload io.Reader) (io.ReadCloser, error)
	clientStream(params string, p *node.Path, ctx context.Context) (<-chan streamEvent, error)
}

func (cn *clientNode) node() node.Node {
	n := &nodeutil.Basic{}
	n.OnBeginEdit = func(r node.NodeRequest) error {
		if !r.EditRoot {
			return nil
		}
		if r.New {
			cn.method = "POST"
		} else {
			cn.method = "PATCH"
		}
		return cn.startEditMode(r.Selection.Path)
	}
	n.OnChild = func(r node.ChildRequest) (node.Node, error) {
		if r.IsNavigation() {
			if valid, err := cn.validNavigation(r.Target); !valid || err != nil {
				return nil, err
			}
			return n, nil
		}
		if r.Delete {
			target := &node.Path{Parent: r.Selection.Path, Meta: r.Meta}
			_, err := cn.request("DELETE", target, nil)
			return nil, err
		}
		if cn.edit != nil {
			return cn.edit.Child(r)
		}
		if IsNil(cn.read) {
			if err := cn.startReadMode(r.Selection.Path); err != nil {
				return nil, err
			}
		}
		return cn.read.Child(r)
	}
	n.OnNext = func(r node.ListRequest) (node.Node, []val.Value, error) {
		if r.IsNavigation() {
			if valid, err := cn.validNavigation(r.Target); !valid || err != nil {
				return nil, nil, err
			}
			return n, r.Key, nil
		}
		if cn.edit != nil {
			return cn.edit.Next(r)
		}
		if IsNil(cn.read) {
			if err := cn.startReadMode(r.Selection.Path); err != nil {
				return nil, nil, err
			}
		}
		return cn.read.Next(r)
	}
	n.OnField = func(r node.FieldRequest, hnd *node.ValueHandle) error {
		if r.IsNavigation() {
			return nil
		} else if cn.edit != nil {
			return cn.edit.Field(r, hnd)
		}
		if IsNil(cn.read) {
			if err := cn.startReadMode(r.Selection.Path); err != nil {
				return err
			}
		}
		return cn.read.Field(r, hnd)
	}
	n.OnNotify = func(r node.NotifyRequest) (node.NotifyCloser, error) {
		var params string // TODO: support params
		ctx, cancel := context.WithCancel(context.Background())
		events, err := cn.support.clientStream(params, r.Selection.Path, ctx)
		if err != nil {
			cancel()
			return nil, err
		}
		go func() {
			for n := range events {
				r.SendWhen(n.Node, n.Timestamp)
			}
		}()
		closer := func() error {
			cancel()
			return nil
		}
		return closer, nil
	}
	n.OnAction = func(r node.ActionRequest) (node.Node, error) {
		return cn.requestAction(r.Selection.Path, r.Input)
	}
	n.OnEndEdit = func(r node.NodeRequest) error {
		// send request
		if !r.EditRoot {
			return nil
		}
		if r.Delete {
			return nil
		}
		_, err := cn.request(cn.method, r.Selection.Path, r.Selection.Split(cn.changes))
		return err
	}
	return n
}

func IsNil(i interface{}) bool {
	if i == nil {
		return true
	}
	return reflect.ValueOf(i).IsNil()
}

func (cn *clientNode) startReadMode(path *node.Path) (err error) {
	cn.read, err = cn.get(path, cn.params)
	return
}

func (cn *clientNode) startEditMode(path *node.Path) error {
	// add depth = 1 so we can pull first level containers and
	// know what container would be conflicts.  we'll have to pull field
	// values too because there's no url param to exclude those yet.
	params := "depth=1&content=config&with-defaults=trim"
	existing, err := cn.get(path, params)
	if err != nil {
		return err
	}
	data := make(map[string]interface{})
	cn.changes = nodeutil.ReflectChild(data)
	cn.edit = &nodeutil.Extend{
		Base: cn.changes,
		OnChild: func(p node.Node, r node.ChildRequest) (node.Node, error) {
			if !r.New && existing != nil {
				return existing.Child(r)
			}
			return p.Child(r)
		},
	}
	return nil
}

func (cn *clientNode) validNavigation(target *node.Path) (bool, error) {
	_, err := cn.request("OPTIONS", target, nil)
	if errors.Is(err, fc.NotFoundError) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return true, nil
}

func (cn *clientNode) get(p *node.Path, params string) (node.Node, error) {
	resp, err := cn.support.clientDo("GET", params, p, nil)
	if err != nil {
		return nil, err
	}
	return jsonNode(resp)
}

func jsonNode(in io.ReadCloser) (node.Node, error) {
	defer in.Close()
	data, err := ioutil.ReadAll(in)
	if err != nil || len(data) == 0 {
		return nil, err
	}
	return nodeutil.ReadJSONIO(bytes.NewBuffer(data)), nil

}

func (cn *clientNode) request(method string, p *node.Path, in *node.Selection) (node.Node, error) {
	var payload bytes.Buffer
	if in != nil {
		if err := in.InsertInto(jsonWtr(cn.compliance, &payload)); err != nil {
			return nil, err
		}
	}
	resp, err := cn.support.clientDo(method, "", p, &payload)
	if err != nil || resp == nil {
		return nil, err
	}
	return jsonNode(resp)
}

func jsonWtr(compliance restconf.ComplianceOptions, out io.Writer) node.Node {
	wtr := &nodeutil.JSONWtr{Out: out, QualifyNamespace: compliance.QualifyNamespaceDisabled}
	return wtr.Node()
}

func (cn *clientNode) requestAction(p *node.Path, in *node.Selection) (node.Node, error) {
	var payload bytes.Buffer
	if in != nil {
		if !cn.compliance.DisableActionWrapper {
			// IETF formated input
			// https://datatracker.ietf.org/doc/html/rfc8040#section-3.6.1

			fmt.Fprintf(&payload, `{"%s:input":`, meta.OriginalModule(p.Meta).Ident())
		}
		if err := in.InsertInto(jsonWtr(cn.compliance, &payload)); err != nil {
			return nil, err
		}
		if !cn.compliance.DisableActionWrapper {
			fmt.Fprintf(&payload, "}")
		}
	}
	resp, err := cn.support.clientDo("POST", "", p, &payload)
	if err != nil {
		return nil, err
	}
	if resp != nil {
		if !cn.compliance.DisableActionWrapper {
			// IETF formated input
			// https://datatracker.ietf.org/doc/html/rfc8040#section-3.6.2
			var vals map[string]interface{}
			d := json.NewDecoder(resp)
			err := d.Decode(&vals)
			if err != nil {
				return nil, err
			}
			a := p.Meta.(*meta.Rpc)
			key := meta.OriginalModule(a).Ident() + ":output"
			respVals, found := vals[key].(map[string]interface{})
			if !found {
				return nil, fmt.Errorf("'%s' missing in output wrapper", key)
			}
			return nodeutil.ReadJSONValues(respVals), nil
		}
		return nodeutil.ReadJSONIO(resp), nil
	}
	return nil, nil
}
