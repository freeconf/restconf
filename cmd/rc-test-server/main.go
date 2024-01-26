package main

import (
	"github.com/freeconf/restconf"
	"github.com/freeconf/restconf/device"
	"github.com/freeconf/yang/fc"
	"github.com/freeconf/yang/source"
	"github.com/freeconf/yang/testdata/car"
)

// Start the car app with RESTCONF Server support to test RESTCONF clients
// against.

func main() {
	fc.DebugLog(true)
	c := car.New()
	api := car.Manage(c)
	ypath := source.Any(
		source.Dir("../../yang"),
		restconf.InternalYPath,
		car.YPath,
	)
	d := device.New(ypath)
	d.Add("car", api)
	restconf.NewServer(d)
	chkerr(d.ApplyStartupConfigFile("startup.json"))
	select {}
}

func chkerr(err error) {
	if err != nil {
		panic(err)
	}
}
