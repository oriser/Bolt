package service

import (
	"context"
	"fmt"
	"time"

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
		woltGroup: g,
	}, nil
}

type groupOrder struct {
	woltGroup     *wolt.Group
	markedAsReady bool
	details       *wolt.OrderDetails
	venue         *wolt.VenueDetails
}

func (g *groupOrder) fetchDetails() (*wolt.OrderDetails, error) {
	details, err := g.woltGroup.Details()
	if err != nil {
		return nil, err
	}
	g.details = details
	return details, nil
}

func (g *groupOrder) fetchVenue() (*wolt.VenueDetails, error) {
	venueDetails, err := g.woltGroup.VenueDetails()
	if err != nil {
		return nil, fmt.Errorf("get venue details: %w", err)
	}
	g.venue = venueDetails
	return venueDetails, nil
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

	status, err := details.Status()
	if err != nil {
		return fmt.Errorf("get status from details: %w", err)
	}

	for status == wolt.StatusActive {
		select {
		case <-time.After(waitBetweenStatusCheck):
			details, err = g.fetchDetails()
			if err != nil {
				return fmt.Errorf("get group details: %w", err)
			}
			status, err = details.Status()
			if err != nil {
				return fmt.Errorf("get status from details: %w", err)
			}
		case <-ctx.Done():
			return fmt.Errorf("context canceled while waiting for group to progress")
		}
	}

	if status == wolt.StatusCanceled {
		return fmt.Errorf("order canceled")
	}

	if status != wolt.StatusPendingTrans && status != wolt.StatusPurchased {
		return fmt.Errorf("unknown order status: %s", status)
	}

	return nil
}

func (g *groupOrder) Details() (*wolt.OrderDetails, error) {
	if g.details == nil {
		return g.fetchDetails()
	}
	return g.details, nil
}

func (g *groupOrder) Venue() (*wolt.VenueDetails, error) {
	if g.venue == nil {
		return g.fetchVenue()
	}
	return g.venue, nil
}

func (g *groupOrder) CalculateDeliveryRate() (int, error) {
	venue, err := g.Venue()
	if err != nil {
		return 0, fmt.Errorf("get venue: %w", err)
	}

	details, err := g.Details()
	if err != nil {
		return 0, fmt.Errorf("get details: %w", err)
	}

	deliveryCoordinate, err := details.DeliveryCoordinate()
	if err != nil {
		return 0, fmt.Errorf("get delivery coordinate: %w", err)
	}

	deliveryPrice, err := venue.CalculateDeliveryRate(deliveryCoordinate)
	if err != nil {
		return 0, fmt.Errorf("get delivery price: %w", err)
	}

	return deliveryPrice, nil
}
