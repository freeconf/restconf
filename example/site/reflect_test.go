package site

import (
	"reflect"
	"testing"
	"time"

	"github.com/freeconf/yang/fc"
	"github.com/freeconf/yang/meta"
	"github.com/freeconf/yang/node"
	"github.com/freeconf/yang/nodeutil"
	"github.com/freeconf/yang/parser"
	"github.com/freeconf/yang/val"
)

type reflectFieldByTypeObj struct {
	LastModified time.Time
}

func TestReflectFieldByType(t *testing.T) {
	myObj := &reflectFieldByTypeObj{}

	//////////////////////////////
	// BEGIN DOC EXAMPLE CODE
	//////////////////////////////

	modStr := `
module anyName {
	leaf lastModified {        
		type int64; // unix seconds
	}
}	
	`
	timeHandler := nodeutil.ReflectField{
		When: nodeutil.ReflectFieldByType(reflect.TypeOf(time.Time{})),
		OnRead: func(leaf meta.Leafable, fieldname string, elem reflect.Value, fieldElem reflect.Value) (val.Value, error) {
			t := fieldElem.Interface().(time.Time)
			return val.Int64(t.Unix()), nil
		},
		OnWrite: func(leaf meta.Leafable, fieldname string, elem reflect.Value, fieldElem reflect.Value, v val.Value) error {
			t := time.Unix(v.Value().(int64), 0)
			fieldElem.Set(reflect.ValueOf(t))
			return nil
		},
	}
	n := nodeutil.Reflect{OnField: []nodeutil.ReflectField{timeHandler}}.Object(myObj)

	//////////////////////////////
	// END DOC EXAMPLE CODE
	//////////////////////////////

	mod, err := parser.LoadModuleFromString(nil, modStr)
	fc.AssertEqual(t, nil, err)
	b := node.NewBrowser(mod, n)
	root := b.Root()
	err = root.UpdateFrom(nodeutil.ReadJSON(`{"lastModified":1000}`)).LastErr
	fc.AssertEqual(t, nil, err)
	fc.AssertEqual(t, int64(1000), myObj.LastModified.Unix())

	actual, err := nodeutil.WriteJSON(root)
	fc.AssertEqual(t, nil, err)
	fc.AssertEqual(t, `{"anyName:lastModified":1000}`, actual)
}

type reflectMap struct {
	Info map[string]interface{}
}

func TestReflectMap(t *testing.T) {
	app := &reflectMap{}

	//////////////////////////////
	// BEGIN DOC EXAMPLE CODE
	//////////////////////////////

	modStr := `
module anyName {
	container info {
		leaf name {
			type string;
		}
		container more {
			leaf size {
				type decimal64;
			}
		}
		list stuff {
			key anything;
			leaf anything {
				type enumeration {
					enum one;
					enum two;
				}
			}
		}
	}
}	
	`

	app.Info = make(map[string]interface{})
	n := nodeutil.ReflectChild(app)
	//////////////////////////////
	// END DOC EXAMPLE CODE
	//////////////////////////////

	mod, err := parser.LoadModuleFromString(nil, modStr)
	fc.AssertEqual(t, nil, err)
	b := node.NewBrowser(mod, n)
	root := b.Root()
	cfg := `{
		"info":{
			"name" : "joe",
			"more" : {
				"size" : 100
			},
			"stuff":[
				{
					"anything" : "one"
				}
			]
		}
	}`
	err = root.UpsertFrom(nodeutil.ReadJSON(cfg)).LastErr
	fc.AssertEqual(t, nil, err)
	roundtrip, err := nodeutil.WriteJSON(root)
	fc.AssertEqual(t, nil, err)
	expected := `{"anyName:info":{"name":"joe","more":{"size":100},"stuff":[{"anything":"one"}]}}`
	fc.AssertEqual(t, expected, roundtrip)
}
