package service

import (
	"context"
	"fmt"
	"math"
	"strings"
	"time"

	"github.com/oriser/bolt/wolt"
)

func (h *Service) buildProgressEmojiArt(startedAt time.Time, deliveryEta time.Time, timezone *time.Location) string {
	const (
		numberOfSpacesBetweenTimes           = 23
		numberOfSpacesBeforeDestinationEmoji = 3
		numberOfRoadTiles                    = 13
		roadTileAsciiArt                     = "_"
		CourierEmoji                         = ":bike:"
		VenueEmoji                           = ":cook:"
	)

	var sb strings.Builder

	// Due to Slack emoji constraints, the courier advances from right (venue) to left (destination)
	firstLine := fmt.Sprintf(
		"`%s`%s`%s`",
		deliveryEta.In(timezone).Format("15:04"),
		strings.Repeat(" ", numberOfSpacesBetweenTimes),
		startedAt.In(timezone).Format("15:04"))
	sb.WriteString(firstLine + "\n")

	deliveryPercentage := time.Now().Sub(startedAt).Seconds() / deliveryEta.Sub(startedAt).Seconds()
	numberOfRoadTilesBehindCourier := int(math.Round(deliveryPercentage * numberOfRoadTiles))
	secondLine := strings.Repeat(" ", numberOfSpacesBeforeDestinationEmoji) +
		fmt.Sprintf(":%s:", h.cfg.OrderDestinationEmoji) +
		strings.Repeat(roadTileAsciiArt, numberOfRoadTiles-numberOfRoadTilesBehindCourier) +
		CourierEmoji +
		strings.Repeat(roadTileAsciiArt, numberOfRoadTilesBehindCourier) +
		VenueEmoji
	sb.WriteString(secondLine)

	return sb.String()
}

func (h *Service) monitorDelivery(initiatedTransport string, order *groupOrder, ctx context.Context, waitBetweenStatusCheck time.Duration, messageID string, originalMessage string) error {
	details, err := order.fetchDetails()
	if err != nil {
		return fmt.Errorf("get group details: %w", err)
	}

	getReadyMessageSent := false
	for details.Status != wolt.StatusCanceled {
		// Check if "delivery_eta" is initialized
		if !details.DeliveryEta.Equal(time.Unix(0, 0)) {
			if !details.PurchaseDatetime.Equal(time.Unix(0, 0)) {
				err = h.eventNotification.EditMessage(
					initiatedTransport,
					strings.TrimSuffix(originalMessage, "\n")+"\n\n"+h.buildProgressEmojiArt(details.PurchaseDatetime, details.DeliveryEta, order.venue.TimezoneLocation),
					order.detailsMessageId)
				if err != nil {
					return fmt.Errorf("updating details message %s: %w", order.detailsMessageId, err)
				}
			}

			timeToDelivery := details.DeliveryEta.Sub(time.Now())
			if !getReadyMessageSent && !details.IsDelivered() && timeToDelivery < h.cfg.TimeTillGetReadyMessage {
				_, _ = h.informEvent(initiatedTransport, "Get ready, delivery coming soon", "", messageID)
				getReadyMessageSent = true
			}
		}

		if details.IsDelivered() {
			if !getReadyMessageSent {
				_, _ = h.informEvent(initiatedTransport, "Delivery arrived", "", messageID)
				getReadyMessageSent = true
			}

			deliveryTime, exists := details.Purchase.DeliveryStatusLog["delivered"]
			if !exists {
				deliveryTime = time.Now()
			}

			err = h.eventNotification.EditMessage(
				initiatedTransport,
				strings.TrimSuffix(originalMessage, "\n")+"\n\n"+h.buildProgressEmojiArt(details.PurchaseDatetime, deliveryTime, order.venue.TimezoneLocation),
				order.detailsMessageId)
			if err != nil {
				return fmt.Errorf("updating delivered details message %s: %w", order.detailsMessageId, err)
			}
			return nil
		}

		select {
		case <-time.After(waitBetweenStatusCheck):
			details, err = order.fetchDetails()
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

	return nil
}
