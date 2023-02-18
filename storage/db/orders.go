package db

import (
	"context"
	"fmt"
	"time"

	sq "github.com/Masterminds/squirrel"
	"github.com/google/uuid"
	"github.com/oriser/bolt/order"
)

type orderModel struct {
	*order.Order
	DBCreatedAt time.Time `db:"db_created_at"`
}

func (d *DBStore) SaveOrder(_ context.Context, order *order.Order) error {
	if order == nil {
		return fmt.Errorf("nil order")
	}
	order.ID = uuid.NewString()
	model := &orderModel{Order: order, DBCreatedAt: time.Now()}

	sql, args, err := sq.Insert("orders").Values(model.ID, model.OriginalID, model.CreatedAt, model.DBCreatedAt,
		model.VenueName, model.VenueID, model.VenueLink, model.VenueCity, model.Host, model.HostID, model.Status, model.DeliveryRate).ToSql()
	if err != nil {
		return fmt.Errorf("generating insert SQL: %w", err)
	}

	if _, err = d.db.Exec(sql, args...); err != nil {
		return newExecError("saving order", sql, err, args...)
	}

	return nil
}
