package db

import (
	"context"
	"fmt"
	"time"

	sq "github.com/Masterminds/squirrel"
	"github.com/google/uuid"
	userDomain "github.com/oriser/bolt/user"
)

type userModel struct {
	*userDomain.User
	CreatedAt time.Time `db:"created_at"`
}

func (d *DBStore) AddUser(_ context.Context, user *userDomain.User) error {
	if user == nil {
		return fmt.Errorf("nil user")
	}
	if user.ID == "" {
		user.ID = uuid.NewString()
	}
	model := &userModel{User: user, CreatedAt: time.Now()}

	sql, args, err := sq.Insert("users").Values(model.ID, model.FullName, model.Email, model.Phone,
		model.Timezone, model.TransportID, model.CreatedAt).ToSql()
	if err != nil {
		return fmt.Errorf("generating insert SQL: %w", err)
	}

	if _, err = d.db.Exec(sql, args...); err != nil {
		return newExecError("adding user", sql, err, args...)
	}

	return nil
}

func (d *DBStore) GetUser(_ context.Context, id string) (*userDomain.User, error) {
	sql, args, err := sq.Select("*").From("users").Where("id=?", id).ToSql()
	if err != nil {
		return nil, fmt.Errorf("generating select SQL: %w", err)
	}

	var user []*userModel
	err = d.db.Select(&user, sql, args...)
	if err != nil {
		return nil, newExecError("selecting user", sql, err, args...)
	}

	if len(user) > 1 {
		return nil, fmt.Errorf("more than one user found (found %d)", len(user))
	}
	if len(user) == 0 {
		return nil, fmt.Errorf("user not found")
	}

	return user[0].User, nil
}

func (d *DBStore) ListUsers(_ context.Context, filter userDomain.ListFilter) ([]*userDomain.User, error) {
	baseSql := sq.Select("*").From("users")

	sqFilter := sq.Or{}
	if len(filter.Names) > 0 {
		sqFilter = append(sqFilter, sq.Eq{"full_name": filter.Names})
	}
	if filter.TransportID != "" {
		sqFilter = append(sqFilter, sq.Eq{"transport_id": filter.TransportID})
	}

	if len(sqFilter) > 0 {
		baseSql = baseSql.Where(sqFilter)
	}

	sql, args, err := baseSql.ToSql()
	if err != nil {
		return nil, fmt.Errorf("generating list SQL: %w", err)
	}

	var users []*userModel
	err = d.db.Select(&users, sql, args...)
	if err != nil {
		return nil, newExecError("selecting users", sql, err, args...)
	}

	ret := make([]*userDomain.User, len(users))
	for i, user := range users {
		ret[i] = user.User
	}

	return ret, nil
}
