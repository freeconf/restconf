package restconf

import (
	"testing"

	"github.com/freeconf/manage/device"
	"github.com/freeconf/manage/gateway"
	"github.com/freeconf/yang/c2"
	"github.com/freeconf/yang/source"
)

func TestCallHome(t *testing.T) {
	c2.DebugLog(true)

	registrar := gateway.NewLocalRegistrar()
	ypath := source.Dir("../yang")
	regDevice := device.New(ypath)
	if err := regDevice.Add("fc-registrar", gateway.RegistrarNode(registrar)); err != nil {
		t.Error(err)
	}
	caller := NewCallHome(func(string) (device.Device, error) {
		return regDevice, nil
	})
	options := caller.Options()
	options.DeviceId = "x"
	options.Address = "north"
	options.LocalAddress = "south"
	var gotUpdate bool
	caller.OnRegister(func(d device.Device, update RegisterUpdate) {
		gotUpdate = true
	})
	caller.ApplyOptions(options)
	if !gotUpdate {
		t.Error("no update recieved")
	}
	c2.AssertEqual(t, 1, registrar.RegistrationCount())
}
