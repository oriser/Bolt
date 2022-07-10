package combined

import (
	"context"
	"fmt"

	userDomain "github.com/oriser/bolt/user"
)

// The UserStoreCombined combines basically just listing users. it takes 2 user stores and do the following:
// 1. For AddUser, adding just to the first
// 2. For GetUser, try to get from the first, if had an error, takes from the second
// 3. For ListUsers, listing the first, then listing the second and combines them

type UserStoreCombined struct {
	first  userDomain.Store
	second userDomain.Store
}

func NewPrioritizedUserStore(first, second userDomain.Store) *UserStoreCombined {
	return &UserStoreCombined{
		first:  first,
		second: second,
	}
}

func (p *UserStoreCombined) AddUser(ctx context.Context, user *userDomain.User) error {
	return p.first.AddUser(ctx, user)
}

func (p *UserStoreCombined) ListUsers(ctx context.Context, filter userDomain.ListFilter) ([]*userDomain.User, error) {
	users, err := p.first.ListUsers(ctx, filter)
	if err != nil {
		return nil, fmt.Errorf("listing users from first storage: %w", err)
	}

	if len(filter.Names) == 1 && filter.TransportID == "" && len(users) == 1 {
		// If we only asked to search for a single user, and we got it from the first storage, no need to list from second storage
		return users, nil
	}

	secondUsers, err := p.second.ListUsers(ctx, filter)
	if err != nil {
		return nil, fmt.Errorf("listing users from second storage: %w", err)
	}
	users = append(users, secondUsers...)
	return users, nil
}

func (p *UserStoreCombined) GetUser(ctx context.Context, id string) (*userDomain.User, error) {
	user, err := p.first.GetUser(ctx, id)
	if err != nil || user == nil {
		user, err = p.second.GetUser(ctx, id)
	}
	return user, err
}
