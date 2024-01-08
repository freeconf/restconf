//go:build ignore
// +build ignore

package main

import (
	"os"
	"text/template"

	"github.com/freeconf/yang/meta"
	"github.com/freeconf/yang/parser"
	"github.com/freeconf/yang/source"
)

var src = `
package estream

const Working = 1

`

// parse proto buf files into Go objects and then call templates to take
// Go objects and generate code based on the data defined in the proto file(s)
func main() {
	ypath := source.Dir("../yang/ietf-rfc")
	m, err := parser.LoadModule(ypath, "ietf-subscribed-notifications")
	chkerr(err)
	t, err := template.New("test").Parse(src)
	chkerr(err)
	vars := struct {
		M *meta.Module
	}{
		M: m,
	}
	t.Execute(os.Stdout, vars)
}

func chkerr(err error) {
	if err != nil {
		panic(err)
	}
}
