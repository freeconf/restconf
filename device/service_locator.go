package device

import "github.com/freeconf/yang/nodeutil"

type ServiceLocator interface {
	Device(id string) (Device, error)
	OnUpdate(l ChangeListener) nodeutil.Subscription
	OnModuleUpdate(module string, l ChangeListener) nodeutil.Subscription
}
