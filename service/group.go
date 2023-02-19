package service

import (
	"context"
	"fmt"
	"time"

	"github.com/oriser/bolt/order"
	"github.com/oriser/bolt/wolt"
)

func (h *Service) joinGroupOrder(groupID string) (*groupOrder, error) {
	g, err := wolt.NewGroupWithExistingID(wolt.WoltAddr{
		BaseAddr:    h.cfg.WoltBaseAddr,
		APIBaseAddr: h.cfg.WoltApiBaseAddr,
	}, wolt.RetryConfig{
		HTTPMaxRetries:       h.cfg.WoltHTTPMaxRetryCount,
		HTTPMinRetryDuration: h.cfg.WoltHTTPMinRetryDuration,
		HTTPMaxRetryDuration: h.cfg.WoltHTTPMaxRetryDuration,
	}, groupID)
	if err != nil {
		return nil, fmt.Errorf("new existing group: %w", err)
	}

	if err := g.Join(); err != nil {
		return nil, fmt.Errorf("join group: %w", err)
	}

	return &groupOrder{
		deliveryPrice: -1,
		id:            groupID,
		woltGroup:     g,
	}, nil
}

type groupOrder struct {
	id            string
	deliveryPrice int
	woltGroup     *wolt.Group
	markedAsReady bool
	details       *wolt.OrderDetails
	venue         *wolt.Venue
}

func (g *groupOrder) fetchDetails() (*wolt.OrderDetails, error) {
	details, err := g.woltGroup.Details()
	if err != nil {
		return nil, err
	}
	g.details = details
	return details, nil
}

func (g *groupOrder) fetchVenue() (*wolt.Venue, error) {
	venue, err := g.woltGroup.VenueDetails()
	if err != nil {
		return nil, fmt.Errorf("get venue details: %w", err)
	}
	g.venue = venue
	return venue, nil
}

func (g *groupOrder) MarkAsReady() error {
	if err := g.woltGroup.MarkAsReady(); err != nil {
		return fmt.Errorf("wolt mark as ready: %w", err)
	}
	g.markedAsReady = true
	return nil
}

func (g *groupOrder) WaitUntilFinished(ctx context.Context, waitBetweenStatusCheck time.Duration) error {
	details, err := g.fetchDetails()
	if err != nil {
		return fmt.Errorf("get group details: %w", err)
	}

	for details.Status == wolt.StatusActive {
		select {
		case <-time.After(waitBetweenStatusCheck):
			details, err = g.fetchDetails()
			if err != nil {
				return fmt.Errorf("get group details: %w", err)
			}
		case <-ctx.Done():
			return fmt.Errorf("context canceled while waiting for group to progress")
		}
	}

	if details.Status == wolt.StatusCanceled {
		return fmt.Errorf("order canceled")
	}

	if !details.Status.Purchased() {
		return fmt.Errorf("unknown order status: %s", details.Status)
	}

	return nil
}

func (g *groupOrder) Details() (*wolt.OrderDetails, error) {
	if g.details == nil {
		return g.fetchDetails()
	}
	return g.details, nil
}

func (g *groupOrder) Venue() (*wolt.Venue, error) {
	if g.venue == nil {
		return g.fetchVenue()
	}
	return g.venue, nil
}

func (g *groupOrder) CalculateDeliveryRate() (int, error) {
	if g.deliveryPrice >= 0 {
		return g.deliveryPrice, nil
	}

	venue, err := g.Venue()
	if err != nil {
		return 0, fmt.Errorf("get venue: %w", err)
	}

	details, err := g.Details()
	if err != nil {
		return 0, fmt.Errorf("get details: %w", err)
	}

	deliveryPrice, err := venue.CalculateDeliveryRate(details.ParsedDeliveryCoordinate)
	if err != nil {
		return 0, fmt.Errorf("get delivery price: %w", err)
	}

	g.deliveryPrice = deliveryPrice
	return deliveryPrice, nil
}

func (g *groupOrder) ToOrder(rates []Rate, receiver string) (*order.Order, error) {
	details, err := g.Details()
	if err != nil {
		return nil, err
	}
	venue, err := g.Venue()
	if err != nil {
		return nil, err
	}

	deliveryPrice, err := g.CalculateDeliveryRate()
	if err != nil {
		return nil, fmt.Errorf("calculate delivery price: %w", err)
	}

	status := order.StatusInvalid
	switch {
	case details.Status == wolt.StatusCanceled:
		status = order.StatusCanceled
	case details.Status.Purchased():
		status = order.StatusDone
	}

	participants := make([]order.Participant, 0, len(rates))
	for _, rate := range rates {
		p := order.Participant{
			Name:   rate.WoltName,
			Amount: rate.Amount,
		}
		if rate.User != nil {
			p.ID = rate.User.ID
		}
		participants = append(participants, p)
	}

	return &order.Order{
		OriginalID:   g.id,
		CreatedAt:    details.CreatedAt,
		Receiver:     receiver,
		VenueName:    venue.Name,
		VenueID:      details.Details.VenueID,
		VenueLink:    venue.Link,
		VenueCity:    venue.City,
		Host:         details.Host,
		HostID:       details.HostID,
		Status:       status,
		Participants: participants,
		DeliveryRate: deliveryPrice,
	}, nil
}
