package main

import (
	"fmt"
	"strings"
	"time"

	"github.com/freeconf/restconf"
	"github.com/freeconf/restconf/device"
	"github.com/freeconf/yang/fc"
	"github.com/freeconf/yang/node"
	"github.com/freeconf/yang/nodeutil"
	"github.com/freeconf/yang/source"
)

// write your code to capture your domain however you want
type Car struct {
	Speed int
	Miles float64
}

func (c *Car) Start() {
	for {
		<-time.After(time.Second)
		c.Miles += float64(c.Speed)
	}
}

// write mangement api to bridge from YANG to code
func manage(car *Car) node.Node {
	return &nodeutil.Extend{

		// use reflect when possible, here we're using to get/set speed AND
		// to read miles metrics.
		Base: nodeutil.ReflectChild(car),

		// handle action request
		OnAction: func(parent node.Node, req node.ActionRequest) (node.Node, error) {
			switch req.Meta.Ident() {
			case "reset":
				car.Miles = 0
				return nil, fmt.Errorf("no no no. %w", fc.UnauthorizedError)
			}
			return nil, nil
		},
	}
}

// Connect everything together into a server to start up
func main() {

	// Your app
	car := &Car{}

	// Device can hold multiple modules, here we are only adding one
	d := device.New(source.Path(".:../yang"))
	if err := d.Add("car", manage(car)); err != nil {
		panic(err)
	}

	// Select wire-protocol RESTCONF to serve the device.
	restconf.NewServer(d)

	// apply start-up config normally stored in a config file on disk
	config := `{
		"fc-restconf":{"web":{"port":":8080"}},
        "car":{"speed":10}
	}`
	if err := d.ApplyStartupConfig(strings.NewReader(config)); err != nil {
		panic(err)
	}

	// start your app
	car.Start()
}
