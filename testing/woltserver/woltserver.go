package woltserver

import (
	"fmt"
	"math/rand"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/gorilla/mux"
)

var ErrNoSuchOrder = fmt.Errorf("no such order. Make sure you created an order first")
var ErrNoSuchParticipant = fmt.Errorf("no such participant. Make sure you created a participant in that order fist")
var ErrNoSuchVenue = fmt.Errorf("no such venue. Make sure you created a venue fist")

type WoltServer struct {
	router       *mux.Router
	server       *httptest.Server
	l            sync.RWMutex
	orders       map[string]*Order // ID to order
	shortIDOrder map[string]string // Order short ID to ID
	venues       map[string]*Venue
	t            *testing.T
}

func NewWoltServer(t *testing.T) *WoltServer {
	rand.Seed(time.Now().UnixNano()) // For random 50x http errors

	router := mux.NewRouter()
	server := httptest.NewUnstartedServer(router)

	ws := &WoltServer{
		router:       router,
		server:       server,
		orders:       make(map[string]*Order),
		shortIDOrder: make(map[string]string),
		venues:       make(map[string]*Venue),
		t:            t,
	}
	ws.registerDefaults()
	return ws
}

func (ws *WoltServer) registerDefaults() {
	defaults := map[string]http.HandlerFunc{
		"/en/group-order/{id}/join":                  ws.joinByShortIDHandler,
		"/v1/group_order/guest/{id}/participants/me": ws.orderDetailsHandler,
		"/v1/group_order/guest/join/{id}":            ws.joinByIDHandler,
		"/v3/venues/{id}":                            ws.getVenueHandler,
	}
	for pattern, handler := range defaults {
		ws.RegisterEndpoint(pattern, handler)
	}
}

func (ws *WoltServer) Addr() string {
	return ws.server.Listener.Addr().String()
}

func (ws *WoltServer) Start() {
	ws.server.Start()
}

func (ws *WoltServer) Stop() {
	ws.server.Close()
}

func (ws *WoltServer) RegisterEndpoint(pattern string, handler http.HandlerFunc) {
	ws.router.HandleFunc(pattern, func(writer http.ResponseWriter, request *http.Request) {
		if 0 == rand.Intn(7) {
			// Randomly return some 502 errors to simulate wolt server errors
			ws.t.Log("Returning 502 error")
			ws.writeError(writer, http.StatusBadGateway, fmt.Errorf("random error"))
			return
		}

		handler.ServeHTTP(writer, request)
	})
}

func (ws *WoltServer) GetOrder(orderID string) (*Order, error) {
	o, ok := ws.getOrderByID(orderID)
	if !ok {
		return nil, ErrNoSuchOrder
	}
	return o, nil
}

func (ws *WoltServer) GetVenue(venueID string) (*Venue, error) {
	v, ok := ws.getVenue(venueID)
	if !ok {
		return nil, ErrNoSuchOrder
	}
	return v, nil
}

func (ws *WoltServer) AddParticipant(orderID string, name string) (string, error) {
	o, ok := ws.getOrderByID(orderID)
	if !ok {
		return "", ErrNoSuchOrder
	}
	p := o.AddParticipant(name)
	return p.ID, nil
}

func (ws *WoltServer) AddParticipantItem(orderID, participantID string, itemAmount int) error {
	o, ok := ws.getOrderByID(orderID)
	if !ok {
		return ErrNoSuchOrder
	}

	p, ok := o.ParticipantByID(participantID)
	if !ok {
		return ErrNoSuchParticipant
	}
	p.AddItem(itemAmount)
	return nil
}

func (ws *WoltServer) ChangeParticipantStatus(orderID, participantID string, status ParticipantStatus) error {
	o, ok := ws.getOrderByID(orderID)
	if !ok {
		return ErrNoSuchOrder
	}

	p, ok := o.ParticipantByID(participantID)
	if !ok {
		return ErrNoSuchParticipant
	}
	p.Status = status
	return nil
}

func (ws *WoltServer) UpdateOrderStatus(orderID string, status OrderStatus) error {
	o, ok := ws.getOrderByID(orderID)
	if !ok {
		return ErrNoSuchOrder
	}
	o.Status = status
	return nil
}

func (ws *WoltServer) CreateOrder(host, venueID string, location Coordinate) (shortID, ID string) {
	o := ws.createOrder(host, venueID, location)
	return o.ShortID, o.ID
}

func (ws *WoltServer) CreateVenue(location Coordinate) string {
	v := ws.createVenue(location)
	return v.ID
}

func (ws *WoltServer) unsafeGetOrderByShortID(shortID string) (*Order, bool, error) {
	orderID, ok := ws.shortIDOrder[shortID]
	if !ok {
		return nil, false, nil
	}

	order, ok := ws.orders[orderID]
	if !ok {
		return nil, false, fmt.Errorf("found order ID but not order")
	}

	return order, true, nil
}

func (ws *WoltServer) getOrderByID(id string) (*Order, bool) {
	ws.l.RLock()
	defer ws.l.RUnlock()
	o, ok := ws.orders[id]
	return o, ok
}

func (ws *WoltServer) getOrderByShortID(shortID string) (*Order, bool, error) {
	ws.l.RLock()
	defer ws.l.RUnlock()
	return ws.unsafeGetOrderByShortID(shortID)
}

func (ws *WoltServer) createOrder(host, venueID string, location Coordinate) *Order {
	ws.l.Lock()
	defer ws.l.Unlock()
	createdOrder := newOrder(host, venueID, location)
	ws.shortIDOrder[createdOrder.ShortID] = createdOrder.ID
	ws.orders[createdOrder.ID] = &createdOrder
	return &createdOrder
}

func (ws *WoltServer) getVenue(id string) (*Venue, bool) {
	ws.l.RLock()
	defer ws.l.RUnlock()

	v, ok := ws.venues[id]
	return v, ok
}

func (ws *WoltServer) createVenue(location Coordinate) *Venue {
	v := newVenue(location)

	ws.l.Lock()
	defer ws.l.Unlock()

	ws.venues[v.ID] = &v
	return &v
}
