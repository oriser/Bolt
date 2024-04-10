package service

import (
	"context"
	"fmt"
	"log"
	"strings"
	"time"
)

func (h *Service) buildClosedVenueMessage(offlinePeriodEnd time.Time, timezone *time.Location) string {
	var sb strings.Builder

	sb.WriteString("Venue is closed for delivery")
	if !IsUnixZero(offlinePeriodEnd) {
		timeFormatString := "{time}"
		if !IsToday(offlinePeriodEnd, timezone) {
			timeFormatString = "{date_num} {time}"
		}
		offlinePeriodEndString := fmt.Sprintf("<!date^%d^%s|%s>", offlinePeriodEnd.Unix(), timeFormatString, offlinePeriodEnd.In(timezone).Format("2006-01-02 15:04"))
		sb.WriteString(fmt.Sprintf(" (allegedly until %s)", offlinePeriodEndString))
	}

	sb.WriteString(" – I'll let you know when it opens")

	return sb.String()
}

func (h *Service) monitorVenue(ctx context.Context, order *groupOrder, receiver, initialMessageID string) {
	details, err := order.Details()
	if err != nil {
		log.Printf("Error getting details for order %q: %v\n", order.id, err)
		return
	}

	waitingToOpenDeliveries := false
	var lastOfflinePeriodEnd time.Time
	var venueClosedMessageId string

	ticker := time.NewTicker(h.cfg.WaitBetweenStatusCheck)

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			venue, err := order.woltGroup.VenueDetails(details)
			if err != nil {
				log.Printf("Error getting venue for order %q: %v\n", order.id, err)
				continue
			}

			if waitingToOpenDeliveries && venue.IsDelivering() {
				_, _ = h.informEvent(receiver, "Venue is now open for delivery", "", initialMessageID)
				waitingToOpenDeliveries = false
			} else if !waitingToOpenDeliveries && !venue.IsDelivering() {
				_, _ = h.informEvent(receiver, h.buildClosedVenueMessage(venue.OfflinePeriodEnd, venue.TimezoneLocation), "", initialMessageID)
				waitingToOpenDeliveries = true
				lastOfflinePeriodEnd = venue.OfflinePeriodEnd
			} else if waitingToOpenDeliveries && lastOfflinePeriodEnd != venue.OfflinePeriodEnd {
				_ = h.eventNotification.EditMessage(receiver, h.buildClosedVenueMessage(venue.OfflinePeriodEnd, venue.TimezoneLocation), venueClosedMessageId)
				lastOfflinePeriodEnd = venue.OfflinePeriodEnd
			}
		}
	}
}
