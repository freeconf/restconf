module github.com/freeconf/restconf

go 1.12

require (
	github.com/freeconf/gconf v0.0.0-20191209144438-2f55915426b9
	github.com/freeconf/yang v0.0.0-20200122003835-a31e8a9b9760
	golang.org/x/net v0.0.0-20200114155413-6afb5195e5aa
)

replace github.com/freeconf/yang => ../yang
