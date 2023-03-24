package site

import (
	"reflect"
	"strings"
	"testing"

	"github.com/freeconf/yang/fc"
	"github.com/freeconf/yang/node"
	"github.com/freeconf/yang/nodeutil"
	"github.com/freeconf/yang/parser"
	"github.com/freeconf/yang/val"
)

type Cub struct {
	Name string
}

type Bear struct {
	Cubs []*Cub
}

func TestBasicReadOnlyList(t *testing.T) {
	manageCub := func(*Cub) node.Node {
		return nil
	}
	var someCubs []*Cub

	//////////////////////////////
	// BEGIN DOC EXAMPLE CODE
	//////////////////////////////

	modStr := `
module bear {
	list cub {
		key name;
		config false;
		leaf name {
			type string;
		}
	}
}	
	`
	bear := &Bear{Cubs: someCubs}
	manageCubs := &nodeutil.Basic{
		OnNext: func(r node.ListRequest) (node.Node, []val.Value, error) {
			key := r.Key
			var found *Cub
			if key != nil {
				name := key[0].String()
				for _, cub := range bear.Cubs {
					if cub.Name == name {
						found = cub
						break
					}
				}
			} else if r.Row < len(bear.Cubs) {
				found = bear.Cubs[r.Row]
				key = []val.Value{val.String(found.Name)}
			}
			if found != nil {
				return manageCub(found), key, nil
			}
			return nil, nil, nil
		},
	}

	//////////////////////////////
	// END DOC EXAMPLE CODE
	//////////////////////////////

	_, err := parser.LoadModuleFromString(nil, modStr)
	fc.AssertEqual(t, nil, err)
	fc.AssertEqual(t, bear, bear)
	fc.AssertEqual(t, manageCubs, manageCubs)
}

func TestBasicEditableList(t *testing.T) {
	manageCub := func(*Cub) node.Node {
		return nil
	}

	//////////////////////////////
	// BEGIN DOC EXAMPLE CODE
	//////////////////////////////

	modStr := `
module bear {
	list cub {
		key name;
		leaf name {
			type string;
		}
	}
}	
	`
	bear := &Bear{}
	manageCubs := &nodeutil.Basic{
		OnNextItem: func(r node.ListRequest) nodeutil.BasicNextItem {
			var found *Cub
			return nodeutil.BasicNextItem{
				New: func() error {
					name := r.Key[0].String()
					found = &Cub{Name: name}
					bear.Cubs = append(bear.Cubs, found)
					return nil
				},
				GetByKey: func() error {
					name := r.Key[0].String()
					for _, cub := range bear.Cubs {
						if cub.Name == name {
							found = cub
							break
						}
					}
					return nil
				},
				DeleteByKey: func() error {
					name := r.Key[0].String()
					copy := make([]*Cub, 0, len(bear.Cubs))
					for _, cub := range bear.Cubs {
						if cub.Name != name {
							copy = append(copy, cub)
						}
					}
					bear.Cubs = copy
					return nil
				},
				GetByRow: func() ([]val.Value, error) {
					var key []val.Value
					if r.Row < len(bear.Cubs) {
						found = bear.Cubs[r.Row]
						key = []val.Value{val.String(found.Name)}
					}
					return key, nil
				},
				Node: func() (node.Node, error) {
					if found != nil {
						return manageCub(found), nil
					}
					return nil, nil
				},
			}
		},
	}

	//////////////////////////////
	// END DOC EXAMPLE CODE
	//////////////////////////////

	_, err := parser.LoadModuleFromString(nil, modStr)
	fc.AssertEqual(t, nil, err)
	fc.AssertEqual(t, bear, bear)
	fc.AssertEqual(t, manageCubs, manageCubs)
}

type Friend struct {
	Name string
}

type Chipmuck struct {
	Friend map[string]*Friend
}

func TestBasicEditableMap(t *testing.T) {
	manageFriend := func(*Friend) node.Node {
		return nil
	}

	//////////////////////////////
	// BEGIN DOC EXAMPLE CODE
	//////////////////////////////

	modStr := `
module chipmuck {
	list friends {
		key name;
		leaf name {
			type string;
		}
	}
}	
	`
	cmunk := &Chipmuck{}
	manageCmunk := &nodeutil.Basic{
		OnNextItem: func(r node.ListRequest) nodeutil.BasicNextItem {
			index := node.NewIndex(cmunk.Friend)
			index.Sort(func(a, b reflect.Value) bool {
				return strings.Compare(a.String(), b.String()) < 0
			})
			var found *Friend
			return nodeutil.BasicNextItem{
				New: func() error {
					name := r.Key[0].String()
					found = &Friend{Name: name}
					cmunk.Friend[name] = found
					return nil
				},
				GetByKey: func() error {
					name := r.Key[0].String()
					found = cmunk.Friend[name]
					return nil
				},
				DeleteByKey: func() error {
					name := r.Key[0].String()
					delete(cmunk.Friend, name)
					return nil
				},
				GetByRow: func() ([]val.Value, error) {
					if r.Row < index.Len() {
						if x := index.NextKey(r.Row); x != node.NO_VALUE {
							name := x.String()
							found = cmunk.Friend[name]
							return []val.Value{val.String(name)}, nil
						}
					}
					return nil, nil
				},
				Node: func() (node.Node, error) {
					if found != nil {
						return manageFriend(found), nil
					}
					return nil, nil
				},
			}
		},
	}

	//////////////////////////////
	// END DOC EXAMPLE CODE
	//////////////////////////////

	_, err := parser.LoadModuleFromString(nil, modStr)
	fc.AssertEqual(t, nil, err)
	fc.AssertEqual(t, cmunk, cmunk)
	fc.AssertEqual(t, manageCmunk, manageCmunk)
}
