package db

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	sq "github.com/Masterminds/squirrel"
	"github.com/google/uuid"
	"github.com/oriser/bolt/order"
)

type orderModel struct {
	*order.Order
	MarshaledParticipants []byte    `db:"participants"`
	DBCreatedAt           time.Time `db:"db_created_at"`
}

func (d *DBStore) SaveOrder(_ context.Context, order *order.Order) error {
	if order == nil {
		return fmt.Errorf("nil order")
	}
	order.ID = uuid.NewString()
	model := &orderModel{Order: order, DBCreatedAt: time.Now()}

	marshaledParticipants, err := json.Marshal(order.Participants)
	if err != nil {
		return fmt.Errorf("marshal participants: %w", err)
	}
	model.MarshaledParticipants = marshaledParticipants

	sql, args, err := sq.Insert("orders").Values(model.ID, model.OriginalID, model.CreatedAt, model.DBCreatedAt, model.Receiver, //nolint // it doesn't recognize the embedded struct
		model.VenueName, model.VenueID, model.VenueLink, model.VenueCity, model.Host, model.HostID, model.Status, model.MarshaledParticipants, model.DeliveryRate).ToSql() // nolint // it doesn't recognize the embedded struct
	if err != nil {
		return fmt.Errorf("generating insert SQL: %w", err)
	}

	if _, err = d.db.Exec(sql, args...); err != nil {
		return newExecError("saving order", sql, err, args...)
	}

	return nil
}

func (d *DBStore) GetVenuesWithMostOrders(startTime time.Time, limit uint64, channelId string, filteredVenueIds []string) ([]order.VenueOrderCount, error) {
	query := sq.Select("venue_id", "venue_name", "venue_link", "COUNT(*) as order_count", "MAX(created_at) as last_created_at").
		From("orders").
		Where(sq.Eq{"receiver": channelId}).
		Where(sq.Eq{"status": order.StatusDone}).
		Where(sq.GtOrEq{"created_at": startTime}).
		GroupBy("venue_id").
		OrderBy("order_count DESC", "last_created_at ASC")
	if len(filteredVenueIds) > 0 {
		query = query.Where(sq.Eq{"venue_id": filteredVenueIds})
	}
	if limit > 0 {
		query = query.Limit(limit)
	}

	sql, args, err := query.ToSql()
	if err != nil {
		return nil, fmt.Errorf("building SELECT query: %w", err)
	}

	var venueOrderCounts []order.VenueOrderCount
	err = d.db.Select(&venueOrderCounts, sql, args...)
	if err != nil {
		return nil, fmt.Errorf("executing SELECT query: %w", err)
	}

	return venueOrderCounts, nil
}

func (d *DBStore) GetHostsWithMostOrders(startTime time.Time, limit uint64, channelId string, filteredHostIds []string) ([]order.HostOrderCount, error) {
	query := sq.Select("host_id", "host", "COUNT(*) as order_count", "MAX(created_at) as last_created_at").
		From("orders").
		Where(sq.Eq{"receiver": channelId}).
		Where(sq.Eq{"status": order.StatusDone}).
		Where(sq.GtOrEq{"created_at": startTime}).
		GroupBy("host_id").
		OrderBy("order_count DESC", "last_created_at ASC")
	if len(filteredHostIds) > 0 {
		query = query.Where(sq.Eq{"host_id": filteredHostIds})
	}
	if limit > 0 {
		query = query.Limit(limit)
	}

	sql, args, err := query.ToSql()
	if err != nil {
		return nil, fmt.Errorf("building SELECT query: %w", err)
	}

	var hostOrderCounts []order.HostOrderCount
	err = d.db.Select(&hostOrderCounts, sql, args...)
	if err != nil {
		return nil, fmt.Errorf("executing SELECT query: %w", err)
	}

	return hostOrderCounts, nil
}

func (d *DBStore) GetActiveChannelIds(lastDateConsideredActive time.Time) ([]string, error) {
	query := sq.Select("receiver").
		From("orders").
		Where(sq.GtOrEq{"created_at": lastDateConsideredActive}).
		Distinct()

	sql, args, err := query.ToSql()
	if err != nil {
		return nil, fmt.Errorf("building SELECT query: %w", err)
	}

	var activeChannelIds []string
	err = d.db.Select(&activeChannelIds, sql, args...)
	if err != nil {
		return nil, fmt.Errorf("executing SELECT query: %w", err)
	}

	return activeChannelIds, nil
}
