package service

import (
	"context"
	"errors"
	"fmt"
	"log"
	"sort"
	"strings"
	"time"

	userDomain "github.com/oriser/bolt/user"
	"github.com/oriser/bolt/wolt"
	"github.com/oriser/regroup"
)

var groupLinkRe = regroup.MustCompile(`\/group\/(?P<id>[A-Z0-9]+?)($|\/$)`)

const (
	MarkAsPaidReaction = "money_mouth_face"
	HostRemoveDebts    = "x"
)

type ParsedWoltGroupID struct {
	ID string `regroup:"id,required"`
}

type Rate struct {
	WoltName string
	User     *userDomain.User
	Amount   float64
}

type GroupRate struct {
	Rates        map[string]float64
	Host         string
	DeliveryRate int
}

type GroupRate2 struct {
	Rates        []Rate
	HostWoltUser string
	HostUser     *userDomain.User
	DeliveryRate int
}

func (g *GroupRate) OrderedRates() []Rate {
	keys := make([]string, 0, len(g.Rates))
	for key := range g.Rates {
		keys = append(keys, key)
	}
	sort.Strings(keys)

	rates := make([]Rate, len(g.Rates))
	for i, key := range keys {
		rates[i] = Rate{
			WoltName: key,
			Amount:   g.Rates[key],
		}
	}
	return rates
}

type groupOrder struct {
	details wolt.OrderDetails
	venue   wolt.VenueDetails
	rates   GroupRate
}

func (h *Service) HandleLinkMessage(req LinksRequest) (string, error) {
	// handle just one link in a message
	groupID := h.getWoltGroupID(req.Links)
	if groupID == nil {
		log.Printf("No wolt links found (%+v)", req.Links)
		return "", nil
	}

	if _, ok := h.currentlyWorkingOrders.Load(groupID.ID); ok {
		log.Println("Already working on order", groupID.ID)
		return "", nil
	}
	h.currentlyWorkingOrders.Store(groupID.ID, true)
	defer h.currentlyWorkingOrders.Delete(groupID.ID)

	if !h.shouldHandleOrder() {
		h.informEvent(req.Channel, "It's too late for me.. I won't join this order :sleeping:", "", req.MessageID)
		return "", nil
	}

	groupRate, err := h.getRateForGroup(req.Channel, groupID.ID, req.MessageID)
	if err != nil {
		if strings.Contains(err.Error(), "order canceled") {
			h.informEvent(req.Channel, fmt.Sprintf("Order for group ID %s was canceled", groupID.ID), "", req.MessageID)
			return "", nil
		}
		log.Printf("Error getting rate for group %s: %v\n", groupID.ID, err)
		h.informEvent(req.Channel, fmt.Sprintf("I had an error getting rate for group ID %s", groupID.ID), "", req.MessageID)
		return "", nil
	}

	ratesMessage := h.buildRatesMessage(groupRate, groupID.ID)
	h.informEvent(req.Channel, ratesMessage, MarkAsPaidReaction, req.MessageID)

	if err := h.addDebts(req.Channel, groupID.ID, groupRate, req.MessageID); err != nil {
		log.Println(fmt.Sprintf("Error adding debts: %s", err.Error()))
		h.informEvent(req.Channel, "I had an error adding debts, I won't track this order", "", req.MessageID)
	}
	return "", nil
}

func (h *Service) getWoltGroupID(links []Link) *ParsedWoltGroupID {
	for _, link := range links {
		if link.Domain != "wolt.com" {
			continue
		}

		parsedWoltLink := &ParsedWoltGroupID{}
		if err := groupLinkRe.MatchToTarget(link.URL, parsedWoltLink); err != nil {
			if !errors.Is(err, &regroup.NoMatchFoundError{}) {
				log.Println("Error matching wolt URL regex:", err)
			}
			continue
		}

		return parsedWoltLink
	}
	return nil
}

func (h *Service) informEvent(receiver, event, reactionEmoji, initialMessageID string) {
	if h.eventNotification == nil {
		return
	}

	messageID, err := h.eventNotification.SendMessage(receiver, event, initialMessageID)
	if err != nil {
		log.Printf("Error informing event to receiver %q: %v\n", receiver, err)
		return
	}

	if reactionEmoji == "" {
		return
	}
	if err = h.eventNotification.AddReaction(receiver, messageID, reactionEmoji); err != nil {
		log.Printf("Error adding reaction to message ID %s:%v\n", messageID, err)
	}
}

func (h *Service) getSortedKeys(m map[string]float64) []string {
	keys := make([]string, 0, len(m))
	for key := range m {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}

func (h *Service) buildGroupRates(woltRates map[string]float64, host string, deliveryRate int) GroupRate2 {
	if _, ok := woltRates[host]; !ok {
		// The host didn't take anything, so he won't be included in the rates, add it here just to fetch his user
		woltRates[host] = 0.0
	}
	sortedKeys := h.getSortedKeys(woltRates)
	groupRate := GroupRate2{
		Rates:        make([]Rate, len(woltRates)),
		HostWoltUser: host,
		DeliveryRate: deliveryRate,
	}

	for i, person := range sortedKeys {
		groupRate.Rates[i] = Rate{
			WoltName: person,
			User:     nil,
			Amount:   woltRates[person],
		}
		users, err := h.userStore.ListUsers(context.Background(), userDomain.ListFilter{Names: []string{person}})
		if err != nil {
			log.Printf("Error getting user %s from storage: %v\n", person, err)
			continue
		}
		if len(users) == 0 {
			log.Printf("User not found %s\n", person)
			continue
		}
		if len(users) != 1 {
			log.Printf("More than one user for %s. Taking first: %#v\n", person, users)
			continue
		}

		if person == host {
			groupRate.HostUser = users[0]
		}
		groupRate.Rates[i].User = users[0]
	}

	return groupRate
}

func (h *Service) buildRatesMessage(groupRate GroupRate2, groupID string) string {
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("Rates for Wolt order ID %s (including %d NIS for delivery):\n", groupID, groupRate.DeliveryRate))

	for _, rate := range groupRate.Rates {
		userID := rate.WoltName
		if rate.User != nil {
			userID = fmt.Sprintf("<@%s>", rate.User.TransportID)
		}

		sb.WriteString(fmt.Sprintf("%s: %.2f\n", userID, rate.Amount))
	}

	host := groupRate.HostWoltUser
	if groupRate.HostUser != nil {
		host = fmt.Sprintf("<@%s>", groupRate.HostUser.TransportID)
	}
	sb.WriteString(fmt.Sprintf("\nPay to: %s\n", host))

	if groupRate.HostUser != nil && len(groupRate.HostUser.PaymentPreferences) > 0 {
		sb.WriteString("Preferred payments methods (in order): ")
		strPayments := make([]string, len(groupRate.HostUser.PaymentPreferences))
		for i, v := range groupRate.HostUser.PaymentPreferences {
			strPayments[i] = v.String()
		}
		sb.WriteString(strings.Join(strPayments, ", "))
		sb.WriteString("\n")
	}

	return sb.String()
}

func (h *Service) shouldHandleOrder() bool {
	if h.dontJoinAfter.IsZero() {
		return true
	}

	currentTime := time.Now()
	if h.dontJoinAfterTZ != nil {
		currentTime = currentTime.In(h.dontJoinAfterTZ)
	}

	if (currentTime.Hour() > h.dontJoinAfter.Hour()) ||
		(currentTime.Hour() == h.dontJoinAfter.Hour() && currentTime.Minute() >= h.dontJoinAfter.Minute()) {
		return false
	}

	return true
}

func (h *Service) waitForGroupProgress(g *wolt.Group) error {
	timeoutTime := time.Now().Add(h.cfg.TimeoutForReady)

	details, err := g.Details()
	if err != nil {
		return fmt.Errorf("get group details: %w", err)
	}
	status, err := details.Status()
	if err != nil {
		return fmt.Errorf("get status from details: %w", err)
	}

	for status == wolt.StatusActive {
		if time.Now().After(timeoutTime) {
			return fmt.Errorf("timeout waiting for group to progress")
		}
		time.Sleep(h.cfg.WaitBetweenStatusCheck)

		details, err = g.Details()
		if err != nil {
			return fmt.Errorf("get group details: %w", err)
		}
		status, err = details.Status()
		if err != nil {
			return fmt.Errorf("get status from details: %w", err)
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

func (h *Service) calculateDeliveryRate(g *wolt.Group, details *wolt.OrderDetails) (int, error) {
	venueDetails, err := g.VenueDetails()
	if err != nil {
		return 0, fmt.Errorf("get venue details: %w", err)
	}

	deliveryCoordinate, err := details.DeliveryCoordinate()
	if err != nil {
		return 0, fmt.Errorf("get delivery coordinate: %w", err)
	}

	deliveryPrice, err := venueDetails.CalculateDeliveryRate(deliveryCoordinate)
	if err != nil {
		return 0, fmt.Errorf("get delivery price: %w", err)
	}

	return deliveryPrice, nil
}

func (h *Service) getRateForGroup(receiver, groupID, messageID string) (GroupRate2, error) {
	g, err := wolt.NewGroupWithExistingID(wolt.WoltAddr{
		BaseAddr:    h.cfg.WoltBaseAddr,
		APIBaseAddr: h.cfg.WoltApiBaseAddr,
	}, wolt.RetryConfig{
		HTTPMaxRetries:       h.cfg.WoltHTTPMaxRetryCount,
		HTTPMinRetryDuration: h.cfg.WoltHTTPMinRetryDuration,
		HTTPMaxRetryDuration: h.cfg.WoltHTTPMaxRetryDuration,
	}, groupID)
	if err != nil {
		return GroupRate2{}, fmt.Errorf("new existing group: %w", err)
	}

	if err := g.Join(); err != nil {
		return GroupRate2{}, fmt.Errorf("join group: %w", err)
	}

	h.informEvent(receiver, fmt.Sprintf("Hey :) Just letting you know I joined the group %s", groupID), "", messageID)

	if err := g.MarkAsReady(); err != nil {
		return GroupRate2{}, fmt.Errorf("mark as ready in group: %w", err)
	}

	if err := h.waitForGroupProgress(g); err != nil {
		return GroupRate2{}, fmt.Errorf("wait for group to progress: %w", err)
	}

	details, err := g.Details()
	if err != nil {
		return GroupRate2{}, fmt.Errorf("get group details for calculating delivery: %w", err)
	}

	rates, err := details.RateByPerson()
	if err != nil {
		return GroupRate2{}, fmt.Errorf("rate by person: %w", err)
	}
	host, err := details.Host()
	if err != nil {
		return GroupRate2{}, fmt.Errorf("group host: %w", err)
	}

	deliveryRate, err := h.calculateDeliveryRate(g, details)
	if err != nil {
		h.informEvent(receiver, "I can't find the delivery rate, I'll publish the rates without including the delivery rate", "", messageID)
		log.Println("Error getting delivery rate:", err)
		return h.buildGroupRates(rates, host, 0), nil
	}

	pricePerPerson := float64(deliveryRate) / float64(len(rates))
	for person, rate := range rates {
		rates[person] = rate + pricePerPerson
	}
	return h.buildGroupRates(rates, host, deliveryRate), nil
}
