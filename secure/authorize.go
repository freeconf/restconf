package secure

import (
	"context"

	"github.com/freeconf/yang/fc"
	"github.com/freeconf/yang/meta"
	"github.com/freeconf/yang/node"
)

var noAccess = &Role{}

type Role struct {
	Id     string
	Access map[string]*AccessControl
}

func NewRole() *Role {
	return &Role{
		Access: make(map[string]*AccessControl),
	}
}

type AccessControl struct {
	Path        string
	Permissions Permission
}

type Permission int

const (
	None Permission = iota
	Read
	Full
)

func (role *Role) CheckListPreConstraints(r *node.ListRequest) (bool, error) {
	requested := Read
	if r.New {
		requested = Full
	}
	return role.check(r.Meta, r.Selection.Context, requested)
}

func (role *Role) CheckContainerPreConstraints(r *node.ChildRequest) (bool, error) {
	requested := Read
	if r.New {
		requested = Full
	}
	return role.check(r.Meta, r.Selection.Context, requested)
}

func (role *Role) CheckFieldPreConstraints(r *node.FieldRequest, hnd *node.ValueHandle) (bool, error) {
	requested := Read
	if r.Write {
		requested = Full
	}
	return role.check(r.Meta, r.Selection.Context, requested)
}

func (role *Role) CheckNotifyFilterConstraints(msg *node.Selection) (bool, error) {
	return role.check(msg.Meta(), msg.Context, Full)
}

type contextKey int

var permKey contextKey = 0

func (role *Role) CheckActionPreConstraints(r *node.ActionRequest) (bool, error) {
	return role.check(r.Meta, r.Selection.Context, Full)
}

func (role *Role) ContextConstraint(s *node.Selection) context.Context {
	if acl, found := role.Access[meta.SchemaPath(s.Meta())]; found {
		return context.WithValue(s.Context, permKey, acl.Permissions)
	}
	return s.Context
}

func (role *Role) check(m meta.Meta, c context.Context, requested Permission) (bool, error) {
	allowed := None
	path := meta.SchemaPath(m)
	if acl, found := role.Access[path]; found {
		allowed = acl.Permissions
	} else if x := c.Value(permKey); x != nil {
		allowed = x.(Permission)
	}
	if requested == Read {
		return allowed >= Read, nil
	}
	if allowed >= requested {
		return true, nil
	}
	return false, fc.UnauthorizedError
}
