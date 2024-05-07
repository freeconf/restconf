package device

import (
	"reflect"
	"strings"

	"github.com/freeconf/yang/meta"
	"github.com/freeconf/yang/node"
	"github.com/freeconf/yang/nodeutil"
	"github.com/freeconf/yang/val"
)

// Implementation of RFC8525

// Export device by it's address so protocol server can serve a device
// often referred to northbound
type ModuleAddresser func(m *meta.Module) string

func LocalDeviceYangLibNode(addresser ModuleAddresser, d Device) node.Node {
	return &nodeutil.Basic{
		OnChild: func(r node.ChildRequest) (node.Node, error) {
			switch r.Meta.Ident() {
			case "modules-state":
				return localYangLibModuleState(addresser, d), nil
			case "yang-library":
				return localYangLibYangLibrary(addresser, d), nil
			}
			return nil, nil
		},
	}
}

func localYangLibModuleState(addresser ModuleAddresser, d Device) node.Node {
	return &nodeutil.Basic{
		OnChild: func(r node.ChildRequest) (node.Node, error) {
			switch r.Meta.Ident() {
			case "module":
				mods := d.Modules()
				if len(mods) > 0 {
					return YangLibModuleList(addresser, mods), nil
				}
			}
			return nil, nil
		},
		OnField: func(r node.FieldRequest, hnd *node.ValueHandle) error {
			return nil
		},
	}
}

func YangLibModuleList(addresser ModuleAddresser, mods map[string]*meta.Module) node.Node {
	index := node.NewIndex(mods)
	index.Sort(func(a, b reflect.Value) bool {
		return strings.Compare(a.String(), b.String()) < 0
	})
	return &nodeutil.Basic{
		OnNext: func(r node.ListRequest) (node.Node, []val.Value, error) {
			key := r.Key
			var m *meta.Module
			if r.Key != nil {
				m = mods[r.Key[0].String()]
			} else {
				if v := index.NextKey(r.Row); v != node.NO_VALUE {
					module := v.String()
					if m = mods[module]; m != nil {
						key = []val.Value{val.String(m.Ident())}
					}
				}
			}
			if m != nil {
				return yangLibModuleHandleNode(addresser, m), key, nil
			}
			return nil, nil, nil
		},
	}
}

func yangLibModuleHandleNode(addresser ModuleAddresser, m *meta.Module) node.Node {
	return &nodeutil.Basic{
		OnChild: func(r node.ChildRequest) (node.Node, error) {
			// deviation
			// submodule
			return nil, nil
		},
		OnField: func(r node.FieldRequest, hnd *node.ValueHandle) error {
			switch r.Meta.Ident() {
			case "name":
				hnd.Val = val.String(m.Ident())
			case "revision":
				if m.Revision() != nil {
					hnd.Val = val.String(m.Revision().Ident())
				}
			case "schema":
				hnd.Val = val.String(addresser(m))
			case "namespace":
				hnd.Val = val.String(m.Namespace())
			case "feature":
			case "conformance-type":
			}
			return nil
		},
	}
}

func localYangLibYangLibrary(addresser ModuleAddresser, d Device) node.Node {
	mods := d.Modules()
	modset := ModuleSet{ident: "all", module: mods}
	modsets := make(map[string]*ModuleSet)
	modsets["all"] = &modset
	return &nodeutil.Basic{
		OnChild: func(r node.ChildRequest) (node.Node, error) {
			switch r.Meta.Ident() {
			case "module-set":
				return YangLibModuleSetList(addresser, modsets), nil
			}
			return nil, nil
		},
		OnField: func(r node.FieldRequest, hnd *node.ValueHandle) error {
			return nil
		},
	}
}

func YangLibModuleSetList(addresser ModuleAddresser, modsets map[string]*ModuleSet) node.Node {
	index := node.NewIndex(modsets)
	index.Sort(func(a, b reflect.Value) bool {
		return strings.Compare(a.String(), b.String()) < 0
	})
	return &nodeutil.Basic{
		OnNext: func(r node.ListRequest) (node.Node, []val.Value, error) {
			key := r.Key
			var ms *ModuleSet
			if r.Key != nil {
				ms = modsets[r.Key[0].String()]
			} else {
				if v := index.NextKey(r.Row); v != node.NO_VALUE {
					module := v.String()
					if ms = modsets[module]; ms != nil {
						key = []val.Value{val.String(ms.Ident())}
					}
				}
			}
			if ms != nil {
				return yangLibModuleSetHandleNode(addresser, ms), key, nil
			}
			return nil, nil, nil
		},
	}
}

func yangLibModuleSetHandleNode(addresser ModuleAddresser, ms *ModuleSet) node.Node {
	return &nodeutil.Basic{
		OnChild: func(r node.ChildRequest) (node.Node, error) {
			switch r.Meta.Ident() {
			case "module":
				mods := ms.Module()
				if len(mods) > 0 {
					return YangLibModuleSetModuleList(addresser, mods), nil
				}
			case "import-only-module":
				mods := ms.ImportOnlyModule()
				if len(mods) > 0 {
					return YangLibModuleSetModuleList(addresser, mods), nil
				}
			}
			return nil, nil
		},
		OnField: func(r node.FieldRequest, hnd *node.ValueHandle) error {
			switch r.Meta.Ident() {
			case "name":
				hnd.Val = val.String(ms.Ident())
			}
			return nil
		},
	}
}

func YangLibModuleSetModuleList(addresser ModuleAddresser, mods map[string]*meta.Module) node.Node {
	index := node.NewIndex(mods)
	index.Sort(func(a, b reflect.Value) bool {
		return strings.Compare(a.String(), b.String()) < 0
	})
	return &nodeutil.Basic{
		OnNext: func(r node.ListRequest) (node.Node, []val.Value, error) {
			key := r.Key
			var m *meta.Module
			if r.Key != nil {
				m = mods[r.Key[0].String()]
			} else {
				if v := index.NextKey(r.Row); v != node.NO_VALUE {
					module := v.String()
					if m = mods[module]; m != nil {
						key = []val.Value{val.String(m.Ident())}
					}
				}
			}
			if m != nil {
				return yangLibModuleSetModuleHandleNode(addresser, m), key, nil
			}
			return nil, nil, nil
		},
	}
}

func yangLibModuleSetModuleHandleNode(addresser ModuleAddresser, m *meta.Module) node.Node {
	return &nodeutil.Basic{
		OnChild: func(r node.ChildRequest) (node.Node, error) {
			// submodule
			return nil, nil
		},
		OnField: func(r node.FieldRequest, hnd *node.ValueHandle) error {
			switch r.Meta.Ident() {
			case "name":
				hnd.Val = val.String(m.Ident())
			case "revision":
				if m.Revision() != nil {
					hnd.Val = val.String(m.Revision().Ident())
				}
			case "namespace":
				hnd.Val = val.String(m.Namespace())
			case "location":
				hnd.Val = val.String(addresser(m))
			case "feature":
			}
			return nil
		},
	}
}

type ModuleSet struct {
	ident            string
	module           map[string]*meta.Module
	importOnlyModule map[string]*meta.Module
}

func (ms *ModuleSet) Ident() string {
	return ms.ident
}

func (ms *ModuleSet) Module() map[string]*meta.Module {
	return ms.module
}
func (ms *ModuleSet) ImportOnlyModule() map[string]*meta.Module {
	return ms.importOnlyModule
}
