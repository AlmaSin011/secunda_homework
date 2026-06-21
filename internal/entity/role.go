package entity

import (
	"errors"
	"fmt"
)

type Role string

const (
	RoleOwner  Role = "owner"
	RoleAdmin  Role = "admin"
	RoleMember Role = "member"
)

func (r Role) String() string { return string(r) }

var ValidRoles = []Role{RoleOwner, RoleAdmin, RoleMember}

var ErrInvalidRole = errors.New("invalid role")

func ParseRole(s string) (Role, error) {
	switch Role(s).toLower() {
	case RoleOwner:
		return RoleOwner, nil
	case RoleAdmin:
		return RoleAdmin, nil
	case RoleMember:
		return RoleMember, nil
	default:
		return "", fmt.Errorf("%w: %q", ErrInvalidRole, s)
	}
}

// IsValid — true для одной из трёх допустимых ролей.
func (r Role) IsValid() bool {
	for _, v := range ValidRoles {
		if r == v {
			return true
		}
	}
	return false
}

func (r Role) CanInvite() bool { return r == RoleOwner || r == RoleAdmin }

func (r Role) CanManageTeam() bool { return r == RoleOwner || r == RoleAdmin }

func (r Role) toLower() Role {
	b := []byte(r)
	for i, c := range b {
		if c >= 'A' && c <= 'Z' {
			b[i] = c + ('a' - 'A')
		}
	}
	return Role(b)
}
