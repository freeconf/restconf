//go:build ignore
// +build ignore

package main

import (
	"strings"

	"github.com/freeconf/restconf"
	"github.com/freeconf/restconf/device"
	"github.com/freeconf/restconf/testdata"
	"github.com/freeconf/yang/source"
)

func main() {
	app := testdata.New()
	ypath := source.Any(restconf.InternalYPath, source.Path("."))
	d := device.New(ypath)
	if err := d.Add("car", testdata.Manage(app)); err != nil {
		panic(err)
	}
	restconf.NewServer(d)
	config := `{
		"fc-restconf":{
			"debug": true,
			"web":{
				"port":":8090"
			}
		},
		"car":{"speed":100}
	}`

	// apply start-up config normally stored in a config file on disk
	if err := d.ApplyStartupConfig(strings.NewReader(config)); err != nil {
		panic(err)
	}
	select {}
}
