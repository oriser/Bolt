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
	Name   string  `json:"name"`
	ID     string  `json:"ID"`
	Amount float64 `json:"amount"`
}

type Order struct {
	ID           string        `db:"id"`
	OriginalID   string        `db:"original_id"`
	CreatedAt    time.Time     `db:"created_at"`
	Receiver     string        `db:"receiver"`
	VenueName    string        `db:"venue_name"`
	VenueID      string        `db:"venue_id"`
	VenueLink    string        `db:"venue_link"`
	VenueCity    string        `db:"venue_city"`
	Host         string        `db:"host"`
	HostID       string        `db:"host_id"`
	Participants []Participant `db:"-"`
	Status       Status        `db:"status"`
	DeliveryRate int           `db:"delivery_rate"`
}

type Store interface {
	SaveOrder(ctx context.Context, order *Order) error
}
