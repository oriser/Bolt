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

type VenueOrderCount struct {
	VenueId       string `db:"venue_id"`
	VenueName     string `db:"venue_name"`
	OrderCount    int    `db:"order_count"`
	VenueLink     string `db:"venue_link"`
	LastCreatedAt string `db:"last_created_at"` // The driver returns this column as a string
}

type MouthsFedCount struct {
	HostId         string `db:"host_id"`
	HostName       string `db:"host"`
	MouthsFedCount int    `db:"mouths_fed_count"`
	LastCreatedAt  string `db:"last_created_at"` // The driver returns this column as a string
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
	GetVenuesWithMostOrders(startTime time.Time, limit uint64, channelId string, filteredVenueIds []string) ([]VenueOrderCount, error)
	GetHostsWithMostMouthsFed(startTime time.Time, limit uint64, channelId string, filteredHostIds []string) ([]MouthsFedCount, error)
	GetActiveChannelIds(lastDateConsideredActive time.Time) ([]string, error)
}
