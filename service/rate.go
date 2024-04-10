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
	"github.com/oriser/regroup"
)

var groupLinkRe = regroup.MustCompile(`\/group\/(?P<id>[A-Z0-9]+?)($|\/$)`)

var errWontJoin = errors.New("wont join because the channel is not accessible")
var errNotInTime = errors.New("order not in tracking time")

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
	Rates        []Rate
	HostWoltUser string
	HostUser     *userDomain.User
	DeliveryRate int
}

func getSortedKeys(m map[string]float64) []string {
	keys := make([]string, 0, len(m))
	for key := range m {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
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
	h.currentlyWorkingOrders.Store(groupID.ID, nil)
	defer h.currentlyWorkingOrders.Delete(groupID.ID)

	err := h.eventNotification.AddReaction(req.Channel, req.MessageID, h.cfg.JoinedOrderEmoji)
	if err != nil {
		return "", errWontJoin
	}

	groupRate, err := h.getRateForGroup(req.Channel, groupID.ID, req.MessageID)
	if err != nil {
		if errors.Is(err, errNotInTime) {
			return "", nil
		}
		if strings.Contains(err.Error(), "order canceled") {
			_, _ = h.informEvent(req.Channel, fmt.Sprintf("Order for group ID %s was canceled", groupID.ID), "", req.MessageID)
			return "", nil
		}
		log.Printf("Error getting rate for group %s: %v\n", groupID.ID, err)
		_, _ = h.informEvent(req.Channel, fmt.Sprintf("I had an error getting rate for group ID %s", groupID.ID), "", req.MessageID)
		return "", nil
	}

	order, _ := h.currentlyWorkingOrders.Load(groupID.ID)
	if order == nil {
		return "", fmt.Errorf("order %s not initialized in map", groupID.ID)
	}

	ratesMessage := h.buildRatesMessage(groupRate, groupID.ID)
	order.(*groupOrder).detailsMessageId, err = h.informEvent(req.Channel, ratesMessage, MarkAsPaidReaction, req.MessageID)
	if err != nil {
		return "", fmt.Errorf("failed sending details message: %w", err)
	}

	if err := h.addDebts(req.Channel, groupID.ID, groupRate, req.MessageID); err != nil {
		log.Println(fmt.Sprintf("Error adding debts: %s", err.Error()))
		_, _ = h.informEvent(req.Channel, "I had an error adding debts, I won't track this order", "", req.MessageID)
	}

	ctx, cancel := context.WithTimeout(context.Background(), h.cfg.OrderDoneTimeout)
	defer cancel()
	if err = h.monitorDelivery(req.Channel, order.(*groupOrder), ctx, h.cfg.WaitBetweenStatusCheck, req.MessageID, ratesMessage); err != nil {
		if strings.Contains(err.Error(), "context canceled while waiting") {
			_, _ = h.informEvent(req.Channel, "Timed out waiting for order to be done", "", req.MessageID)
			return "", nil
		}
		return "", fmt.Errorf("error in waiting for order to finish: %w", err)
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

func (h *Service) buildGroupRates(woltRates map[string]float64, host string, deliveryRate int) GroupRate {
	if _, ok := woltRates[host]; !ok {
		// The host didn't take anything, so he won't be included in the rates, add it here just to fetch his user
		woltRates[host] = 0.0
	}
	sortedKeys := getSortedKeys(woltRates)
	groupRate := GroupRate{
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

func (h *Service) buildRatesMessage(groupRate GroupRate, groupID string) string {
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("Rates for Wolt order ID %s (including %d NIS for delivery):\n", groupID, groupRate.DeliveryRate))

	for _, rate := range groupRate.Rates {
		userID := rate.WoltName
		if rate.User != nil {
			userID = fmt.Sprintf("<@%s> (%s)", rate.User.TransportID, rate.WoltName)
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

func (h *Service) saveOrderAsync(order *groupOrder, groupRate GroupRate, receiver string) {
	domainOrder, err := order.ToOrder(groupRate.Rates, receiver)
	if err != nil {
		log.Printf("Error converting order %q: %v\n", order.id, err)
		return
	}
	if err = h.orderStore.SaveOrder(context.Background(), domainOrder); err != nil {
		log.Printf("Error saving order %q: %v\n", order.id, err)
		return
	}

}

func (h *Service) getRateForGroup(receiver, groupID, messageID string) (groupRate GroupRate, err error) {
	shouldHandleOrder := h.shouldHandleOrder()

	if !shouldHandleOrder {
		_, err := h.informEvent(receiver, "It's too late for me... I won't track prices for this order :sleeping:", "", messageID)
		if err != nil {
			return GroupRate{}, errWontJoin
		}
	}

	order, err := h.joinGroupOrder(groupID)
	if err != nil {
		_, _ = h.informEvent(receiver, "I had an error joining the order", "", messageID)
		return GroupRate{}, fmt.Errorf("join group order: %w", err)
	}
	h.currentlyWorkingOrders.Store(groupID, order)

	defer func() {
		go h.saveOrderAsync(order, groupRate, receiver)
	}()

	if err = order.MarkAsReady(); err != nil {
		return GroupRate{}, fmt.Errorf("mark as ready in group: %w", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), h.cfg.TimeoutForReady)
	defer cancel()

	monitorCtx, monitorCancel := context.WithCancel(ctx)
	go h.monitorVenue(monitorCtx, order, receiver, messageID)
	if err = h.WaitUntilFinished(order, ctx); err != nil {
		monitorCancel()
		return GroupRate{}, fmt.Errorf("wait for group to finish: %w", err)
	}
	monitorCancel()

	if !shouldHandleOrder {
		return GroupRate{}, errNotInTime
	}

	details, err := order.Details()
	if err != nil {
		return GroupRate{}, fmt.Errorf("get group details for calculating rates: %w", err)
	}

	rates, err := details.RateByPerson()
	if err != nil {
		return GroupRate{}, fmt.Errorf("rate by person: %w", err)
	}

	deliveryRate, err := order.CalculateDeliveryRate()
	if err != nil {
		_, _ = h.informEvent(receiver, "I can't find the delivery rate, I'll publish the rates without including the delivery rate", "", messageID)
		log.Println("Error getting delivery rate:", err)
		return h.buildGroupRates(rates, details.Host, 0), nil
	}

	pricePerPerson := float64(deliveryRate) / float64(len(rates))
	for person, rate := range rates {
		rates[person] = rate + pricePerPerson
	}
	return h.buildGroupRates(rates, details.Host, deliveryRate), nil
}

func (h *Service) monitorVenue(ctx context.Context, order *groupOrder, receiver, initialMessageID string) {
	details, err := order.Details()
	if err != nil {
		log.Printf("Error getting details for order %q: %v\n", order.id, err)
		return
	}

	// TODO: Configure that
	ticker := time.NewTicker(30 * time.Second)

	online := true
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

			if !venue.Online && online {
				_, _ = h.informEvent(receiver, ":red_circle: Pay attention. The venue went offline :(", "", initialMessageID)
				online = false
			}

			if venue.Online && !online {
				_, _ = h.informEvent(receiver, ":large_green_circle: The venue is back online :)", "", initialMessageID)
				online = true
			}

		}
	}
}
