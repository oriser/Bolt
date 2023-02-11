package service

import (
	"context"
	"errors"
	"fmt"
	"log"
	"time"

	debtDomain "github.com/oriser/bolt/debt"
	userDomain "github.com/oriser/bolt/user"
	"github.com/oriser/regroup"
)

var groupFromMessageRe = regroup.MustCompile(`Wolt order ID (?P<id>[A-Z0-9]+?)[\s\.$]`)

const NoMessagesAfterHour = 21
const NoMessagesBeforeHour = 9

func (h *Service) HandleReactionAdded(req ReactionAddRequest) (string, error) {
	if h.debtStore == nil {
		return "", nil
	}
	// Pay attention that I may get notified about any reaction (to any message), so react just for those came to message from my user ID
	if (req.Reaction != MarkAsPaidReaction && req.Reaction != HostRemoveDebts) || req.MessageUserID != h.selfID {
		return "", nil
	}

	parsedID := &ParsedWoltGroupID{}
	if err := groupFromMessageRe.MatchToTarget(req.MessageText, parsedID); err != nil {
		if errors.Is(err, &regroup.NoMatchFoundError{}) {
			// React to non rates message
			log.Println("Got reaction for non rates message, ignoring")
			return "", nil
		}
		return "", fmt.Errorf("regroup match to target: %w", err)
	}

	switch req.Reaction {
	case MarkAsPaidReaction:
		if err := h.markDebtAsPaid(parsedID.ID, req.FromUserID, req.Channel); err != nil {
			log.Println(fmt.Sprintf("Error marking debt as paid from reaction event: %s", err.Error()))
		}
		return "", nil
	case HostRemoveDebts:
		hostForOrder, err := h.hostForOrderID(parsedID.ID)
		if err != nil {
			log.Println("Error getting host for order ID:", err)
			return "", nil
		}
		if hostForOrder == "" {
			return "", nil
		}

		hostUser, err := h.userStore.GetUser(context.Background(), hostForOrder)
		if err != nil {
			log.Println("Error GetUser for host for order ID:", err)
			return "", nil
		}
		if hostUser.TransportID != req.FromUserID {
			h.informEvent(req.FromUserID, fmt.Sprintf("Nice try :stuck_out_tongue_winking_eye: Only the host (<@%s>) can cancel debts for this order", hostForOrder), "", "")
			return "", nil
		}
		if err := h.removeAllDebtsForOrder(parsedID.ID, "the host requested to cancel debts tracking"); err != nil {
			log.Println(fmt.Sprintf("Error removing all debts for order ID %s: %v", parsedID.ID, err))
		}
	}

	return "", nil
}

func (h *Service) DebtWorker(ctx context.Context, orderID string) {
	if h.debtStore == nil {
		return
	}

	reminderInterval := time.NewTicker(h.cfg.DebtReminderInterval)
	defer reminderInterval.Stop()

	for {
		select {
		case <-reminderInterval.C:
			debts, err := h.debtStore.ListDebtsForOrderID(orderID)
			if err != nil {
				log.Println("Error listing debts:", err)
				continue
			}
			if len(debts) == 0 {
				// No more debts
				return
			}
			for _, debt := range debts {
				if err := h.remindDebt(debt); err != nil {
					log.Printf("Reminding about debt: %#v; error: %v\n", debt, err)
				}
			}
		case <-ctx.Done():
			if err := h.removeAllDebtsForOrder(orderID, "timeout has been reached"); err != nil {
				log.Println("Error removing all debts on context cancellation:", err)
			}
			return
		}
	}
}

func (h *Service) remindDebt(debt *debtDomain.Debt) error {
	borrower, err := h.userStore.GetUser(context.Background(), debt.BorrowerID)
	if err != nil {
		return fmt.Errorf("get borrower user: %w", err)
	}

	timeAtBorrower := time.Now()
	if borrower.Timezone != "" {
		tz, err := time.LoadLocation(borrower.Timezone)
		if err == nil {
			timeAtBorrower = timeAtBorrower.In(tz)
		}
	}

	if timeAtBorrower.Hour() >= NoMessagesAfterHour || timeAtBorrower.Hour() < NoMessagesBeforeHour {
		log.Printf("Not reminding in aftertimes for user %q (%s). Timezone at borrower: %s\n", borrower.FullName, borrower.ID, borrower.Timezone)
		return nil
	}

	h.informEvent(borrower.TransportID,
		fmt.Sprintf("Reminder, you should pay %.2f nis to <@%s> for Wolt order ID %s.\n"+
			"If you paid, you can mark yourself as paid by adding :%s: reaction to this message \\ the original rates message.",
			debt.Amount, debt.LenderID, debt.OrderID, MarkAsPaidReaction),
		MarkAsPaidReaction, "")
	return nil
}

func (h *Service) createDebt(amount float64, initiatedTransport, orderID, messageID string, borrowerUser *userDomain.User, lenderUser *userDomain.User) error {
	if h.debtStore == nil {
		return nil
	}

	debt := debtDomain.NewDebt(borrowerUser.ID, lenderUser.ID, orderID, initiatedTransport, messageID, amount)
	if err := h.debtStore.AdDebt(debt); err != nil {
		return fmt.Errorf("add debt: %w", err)
	}

	return nil
}

func (h *Service) addDebts(initiatedTransport, orderID string, rates GroupRate2, messageID string) error {
	if h.debtStore == nil {
		return nil
	}

	if rates.HostUser == nil {
		h.informEvent(initiatedTransport, fmt.Sprintf("I didn't find the user of the host (%s), I won't track debts for order %s", rates.HostWoltUser, orderID), "", messageID)
		return nil
	}

	h.informEvent(initiatedTransport,
		fmt.Sprintf("I'll keep reminding you to pay, when you pay you can react with :%s: to the rates message and I'll stop bothering you.\n"+
			"<@%s>, as the host, you can react with :%s: to the rates message to cancel debts tracking for Wolt order ID %s",
			MarkAsPaidReaction, rates.HostUser.TransportID, HostRemoveDebts, orderID),
		"", messageID)

	for _, rate := range rates.Rates {
		if rate.WoltName == rates.HostWoltUser {
			// Don't create debt for the lender
			continue
		}

		if rate.User == nil {
			h.informEvent(initiatedTransport, fmt.Sprintf("I won't track %q payment because I can't find his user.", rate.WoltName), "", messageID)
			continue
		}
		if err := h.createDebt(rate.Amount, initiatedTransport, orderID, messageID, rate.User, rates.HostUser); err != nil {
			log.Println(fmt.Sprintf("Error creating debt for user %q in order ID %q: %v", rate.WoltName, orderID, err))
			continue
		}
	}

	//goland:noinspection ALL
	ctx, _ := context.WithTimeout(context.Background(), h.cfg.DebtMaximumDuration) // nolint
	go h.DebtWorker(ctx, orderID)

	return nil
}

func (h *Service) hostForOrderID(orderID string) (string, error) {
	debts, err := h.debtStore.ListDebtsForOrderID(orderID)
	if err != nil {
		return "", fmt.Errorf("list debts: %w", err)
	}
	if len(debts) == 0 {
		return "", nil
	}
	return debts[0].LenderID, nil
}

func (h *Service) removeAllDebtsForOrder(orderID, reason string) error {
	if h.debtStore == nil {
		return nil
	}

	debts, err := h.debtStore.ListDebtsForOrderID(orderID)
	if err != nil {
		return fmt.Errorf("list debts: %w", err)
	}
	if len(debts) == 0 {
		return nil
	}

	lender := debts[0].LenderID
	for _, debt := range debts {
		if err := h.debtStore.RemoveDebtInOrderID(orderID, debt.ID); err != nil {
			return fmt.Errorf("remove debt: %w", err)
		}
	}

	h.informEvent(lender, fmt.Sprintf("I removed all debts for order ID %s because %s", orderID, reason), "", "")
	return nil
}

func (h *Service) markDebtAsPaid(orderID, reactedTransportID, initialChannel string) error {
	if h.debtStore == nil {
		return nil
	}

	debts, err := h.debtStore.ListDebtsForOrderID(orderID)
	if err != nil {
		return fmt.Errorf("list debts: %w", err)
	}
	if len(debts) == 0 {
		return nil
	}

	for _, debt := range debts {
		borrower, err := h.userStore.GetUser(context.Background(), debt.BorrowerID)
		if err != nil {
			log.Println(fmt.Sprintf("Error getting borrower user with id %s: %s", debt.BorrowerID, err.Error()))
			continue
		}
		if borrower.TransportID != reactedTransportID {
			// The reacted user is not the user owned the debt
			continue
		}

		if err := h.debtStore.RemoveDebtInOrderID(orderID, debt.ID); err != nil {
			return fmt.Errorf("remove debt: %w", err)
		}

		h.informEvent(borrower.TransportID, fmt.Sprintf("OK! I removed your debt for order %s", debt.OrderID), "", "")

		// Notify in the initial channel of the wolt link message in case we will get error getting the host details
		recipient := initialChannel
		messageID := debt.MessageID
		lender, err := h.userStore.GetUser(context.Background(), debt.LenderID)
		if err != nil {
			log.Println(fmt.Sprintf("Error getting lender user with id %s: %s", debt.LenderID, err.Error()))
		} else {
			recipient = lender.TransportID
			messageID = ""
		}

		h.informEvent(recipient, fmt.Sprintf("<@%s> marked himself as paid for order ID %s", borrower.TransportID, debt.OrderID), "", messageID)
		return nil
	}

	return nil
}
