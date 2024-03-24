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

	// Use Slack's date formatting to display times at the recipient's timezone
	deliveryEtaString := fmt.Sprintf("<!date^%d^{time}|%s>", deliveryEta.Unix(), deliveryEta.In(timezone).Format("15:04"))
	startedAtString := fmt.Sprintf("<!date^%d^{time}|%s>", startedAt.Unix(), startedAt.In(timezone).Format("15:04"))

	// Due to Slack emoji constraints, the courier advances from right (venue) to left (destination)
	firstLine := fmt.Sprintf(
		"`%s`%s`%s`",
		deliveryEtaString,
		strings.Repeat(" ", numberOfSpacesBetweenTimes),
		startedAtString)
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

func (h *Service) updateDeliveryProgressMessage(initiatedTransport string, order *groupOrder, details *wolt.OrderDetails, ratesMessage string) error {
	var err error

	if details.PurchaseDatetime.Equal(time.Unix(0, 0)) {
		return nil
	}

	var deliveryTime time.Time
	var deliveryTimeExists bool
	if details.IsDelivered() {
		deliveryTime, deliveryTimeExists = details.Purchase.DeliveryStatusLog["delivered"]
		if !deliveryTimeExists {
			deliveryTime = time.Now()
		}
	} else if !details.DeliveryEta.Equal(time.Unix(0, 0)) {
		deliveryTime = details.DeliveryEta
	} else {
		// No delivery ETA yet. Nothing to update.
		return nil
	}

	err = h.eventNotification.EditMessage(
		initiatedTransport,
		strings.TrimSuffix(ratesMessage, "\n")+"\n\n"+h.buildProgressEmojiArt(details.PurchaseDatetime, deliveryTime, order.venue.TimezoneLocation),
		order.detailsMessageId)
	if err != nil {
		return fmt.Errorf("updating details message %s: %w", order.detailsMessageId, err)
	}

	return err
}

func (h *Service) monitorDelivery(initiatedTransport string, order *groupOrder, ctx context.Context, waitBetweenStatusCheck time.Duration, messageID string, ratesMessage string) error {
	details, err := order.fetchDetails()
	if err != nil {
		return fmt.Errorf("get group details: %w", err)
	}

	getReadyMessageSent := false
	for details.Status != wolt.StatusCanceled {
		err = h.updateDeliveryProgressMessage(initiatedTransport, order, details, ratesMessage)
		if err != nil {
			return err
		}

		if details.IsDelivered() {
			if !getReadyMessageSent {
				_, _ = h.informEvent(initiatedTransport, "Delivery arrived", "", messageID)
				getReadyMessageSent = true
			}
			return nil
		} else if !details.DeliveryEta.Equal(time.Unix(0, 0)) {
			timeToDelivery := details.DeliveryEta.Sub(time.Now())
			if !getReadyMessageSent && timeToDelivery < h.cfg.TimeTillGetReadyMessage {
				_, _ = h.informEvent(initiatedTransport, "Get ready, delivery coming soon", "", messageID)
				getReadyMessageSent = true
			}
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
