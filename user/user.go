package user

import (
	"context"
	"fmt"
)

type User struct {
	ID                 string `db:"id"`
	FullName           string `db:"full_name"`
	Email              string `db:"email"`
	Phone              string `db:"phone"`
	PaymentPreferences []PaymentMethod
	Timezone           string `db:"timezone"`
	TransportID        string `db:"transport_id"` // For example slack user ID
}

type ErrNotFound struct {
	Name string
}

func (u *ErrNotFound) Error() string {
	return fmt.Sprintf("user with name %s not found", u.Name)
}

type Store interface {
	AddUser(ctx context.Context, user *User) error
	GetUser(ctx context.Context, id string) (*User, error)
	ListUsers(ctx context.Context, filter ListFilter) ([]*User, error)
}

type ListFilter struct {
	Names       []string
	TransportID string
}
