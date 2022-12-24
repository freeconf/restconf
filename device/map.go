package device

// Map is used my server to host multiple devices in a single web server
// at restconf=[device]/...
type Map interface {
	Device(deviceId string) (Device, error)
}
