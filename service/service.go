package service

import (
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/oriser/bolt/debt"
	"github.com/oriser/bolt/order"
	"github.com/oriser/bolt/user"
)

type EventNotification interface {
	SendMessage(receiver, event, messageID string) (string, error)
	AddReaction(receiver, messageID, reaction string) error
}

type Config struct {
	TimeoutForReady          time.Duration `env:"ORDER_READY_TIMEOUT" envDefault:"40m"`
	TimeoutForDeliveryRate   time.Duration `env:"GET_DELIVERY_RATE_TIMEOUT" envDefault:"10m"`
	WaitBetweenStatusCheck   time.Duration `env:"WAIT_BETWEEN_STATUS_CHECK" envDefault:"20s"`
	DebtReminderInterval     time.Duration `env:"DEBT_REMINDER_INTERVAL" envDefault:"3h"`
	DebtMaximumDuration      time.Duration `env:"DEBT_MAXIMUM_DURATION" envDefault:"24h"`
	DontJoinAfter            string        `env:"DONT_JOIN_AFTER"`
	DontJoinAfterTZ          string        `env:"DONT_JOIN_AFTER_TZ"`
	WoltBaseAddr             string        `env:"WOLT_BASE_ADDR" envDefault:"https://wolt.com"`
	WoltApiBaseAddr          string        `env:"WOLT_API_BASE_ADDR" envDefault:"https://restaurant-api.wolt.com"`
	WoltHTTPMaxRetryCount    int           `env:"WOLT_HTTP_MAX_RETRY_COUNT" envDefault:"5"`
	WoltHTTPMinRetryDuration time.Duration `env:"WOLT_HTTP_MIN_RETRY_DURATION" envDefault:"1s"`
	WoltHTTPMaxRetryDuration time.Duration `env:"WOLT_HTTP_MAX_RETRY_DURATION" envDefault:"30s"`
}

type Service struct {
	cfg                    Config
	eventNotification      EventNotification
	currentlyWorkingOrders sync.Map
	userStore              user.Store
	debtStore              debt.Store
	orderStore             order.Store
	selfID                 string
	dontJoinAfter          time.Time
	dontJoinAfterTZ        *time.Location
}

type ReactionAddRequest struct {
	Reaction      string
	FromUserID    string
	Channel       string
	MessageUserID string
	MessageText   string
}

type Link struct {
	Domain string
	URL    string
}
type LinksRequest struct {
	Links     []Link
	MessageID string
	Channel   string
}

func New(cfg Config, userStore user.Store, debtStore debt.Store, orderStore order.Store, selfID string, eventNotification EventNotification) (*Service, error) {
	var dontJoinAfter time.Time
	var err error
	if cfg.DontJoinAfter != "" {
		dontJoinAfter, err = time.Parse("15:04", cfg.DontJoinAfter)
		if err != nil {
			return nil, fmt.Errorf("parsing DONT_JOIN_AFTER (HH:MM format): %w", err)
		}
	}

	var dontJoinAfterTZ *time.Location
	if cfg.DontJoinAfterTZ != "" {
		dontJoinAfterTZ, err = time.LoadLocation(cfg.DontJoinAfterTZ)
		if err != nil {
			return nil, fmt.Errorf("parsing DONT_JOIN_AFTER_TZ: %w", err)
		}
	}
	return &Service{
		cfg:               cfg,
		eventNotification: eventNotification,
		userStore:         userStore,
		debtStore:         debtStore,
		orderStore:        orderStore,
		selfID:            selfID,
		dontJoinAfter:     dontJoinAfter,
		dontJoinAfterTZ:   dontJoinAfterTZ,
	}, nil
}

func (h *Service) informEvent(receiver, event, reactionEmoji, initialMessageID string) bool {
	if h.eventNotification == nil {
		return true
	}

	messageID, err := h.eventNotification.SendMessage(receiver, event, initialMessageID)
	if err != nil {
		log.Printf("Error informing event to receiver %q: %v\n", receiver, err)
		return false
	}

	if reactionEmoji == "" {
		return true
	}
	if err = h.eventNotification.AddReaction(receiver, messageID, reactionEmoji); err != nil {
		log.Printf("Error adding reaction to message ID %s:%v\n", messageID, err)
	}
	return true
}
