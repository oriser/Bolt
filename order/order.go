package order

import (
	"context"
	"time"
)

type Status int

const (
	StatusInvalid Status = iota
	StatusCanceled
	StatusDone
)

type Participant struct {
	Name   string
	ID     string
	Amount int
}

type Order struct {
	ID           string    `db:"id"`
	Time         time.Time `db:"time"`
	VenueName    string    `db:"venue_name"`
	VenueID      string    `db:"venue_id"`
	VenueLink    string    `db:"venue_link"`
	VenueCity    string    `db:"venue_city"`
	Host         string    `db:"host"`
	HostID       string    `db:"host_id"`
	Status       Status    `db:"status"`
	DeliveryRate int       `db:"delivery_rate"`
}

type Store interface {
	SaveOrder(ctx context.Context, order *Order) error
}
