package woltserver

import (
	"math/rand"
	"sync"
	"time"

	"github.com/oriser/bolt/testing/utils"
)

type OrderStatus string

const (
	StatusActive       OrderStatus = "active"
	StatusCanceled     OrderStatus = "cancelled"
	StatusPendingTrans OrderStatus = "pending_transaction"
	StatusPurchased    OrderStatus = "purchased"
)

type DeliveryMethod string

const (
	DeliveryMethodHome      DeliveryMethod = "homedelivery"
	DeliveryMethodTakeaways DeliveryMethod = "takeaway"
)

type ParticipantStatus string

const (
	ParticipantStatusReady  ParticipantStatus = "ready"
	ParticipantStatusJoined ParticipantStatus = "joined"
)

type Item struct {
	BasePrice int
	EndAmount int
}

type Participant struct {
	FirstName string
	ID        string
	Status    ParticipantStatus
	Items     []Item
	l         sync.Mutex
}

type Order struct {
	ID             string
	ShortID        string
	VenueID        string
	Host           string
	Status         OrderStatus
	Location       Coordinate
	DeliveryMethod DeliveryMethod
	Participants   []*Participant
	participantsID map[string]*Participant
	l              sync.RWMutex
}

type Coordinate struct {
	Lat float64 // 34.77900266647339
	Lon float64 // 32.072447331148844
}

type Venue struct {
	ID       string
	Location Coordinate
}

func init() {
	rand.Seed(time.Now().UnixNano())
}

func generateWoltID() string {
	return utils.GenerateRandomString(append(utils.LowerLetters, utils.NumberLetters...), 24)
}

func generateWoltShortID() string {
	return utils.GenerateRandomString(append(utils.CapitalLetters, utils.NumberLetters...), 8)
}

func (o *Order) AddParticipant(name string) *Participant {
	o.l.Lock()
	defer o.l.Unlock()

	p := newParticipant(name)
	o.Participants = append(o.Participants, &p)
	o.participantsID[p.ID] = &p
	return &p
}

func (o *Order) ParticipantByID(id string) (*Participant, bool) {
	o.l.RLock()
	defer o.l.RUnlock()
	p, ok := o.participantsID[id]
	return p, ok
}

func (p *Participant) AddItem(amount int) {
	p.l.Lock()
	defer p.l.Unlock()
	p.Items = append(p.Items, newItem(amount, amount))
}

func newOrder(host, venueID string, location Coordinate) Order {
	hostID := "a1ufke9dwe2wkn7pmw6qcwx9"
	hostParticipant := newParticipant(host)
	hostParticipant.ID = hostID
	return Order{
		ID:             generateWoltID(),
		ShortID:        generateWoltShortID(),
		VenueID:        venueID,
		Host:           host,
		Location:       location,
		Status:         StatusActive,
		DeliveryMethod: DeliveryMethodHome,
		Participants: []*Participant{
			&hostParticipant,
		},
		participantsID: make(map[string]*Participant),
	}
}

func newParticipant(firstName string) Participant {
	return Participant{
		FirstName: firstName,
		ID:        generateWoltID(),
		Status:    ParticipantStatusJoined,
		Items:     make([]Item, 0),
	}
}

func newItem(basePrice, endAmount int) Item {
	return Item{
		BasePrice: basePrice,
		EndAmount: endAmount,
	}
}

func newVenue(location Coordinate) Venue {
	return Venue{
		ID:       generateWoltID(),
		Location: location,
	}
}
