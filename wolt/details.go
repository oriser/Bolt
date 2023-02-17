package wolt

import (
	"encoding/json"
	"fmt"
)

type DeliveryInfo struct {
	Location struct {
		Coordinates struct {
			Coordinates []float64 `json:"coordinates"`
		} `json:"coordinates"`
	} `json:"location"`
}

type Item struct {
	BasePrice float64 `json:"baseprice"`
	EndAmount float64 `json:"end_amount"`
}

type Participant struct {
	FirstName string `json:"first_name"`
	LastName  string `json:"last_name"`
	Status    string `json:"status"`
	UserID    string `json:"user_id"`
	Basket    struct {
		Items []Item `json:"items"`
	} `json:"basket"`
}

func (p *Participant) Name() string {
	if p.LastName != "" {
		return fmt.Sprintf("%s %s", p.FirstName, p.LastName)
	}
	return p.FirstName
}

type OrderDetails struct {
	Status  string `json:"status"`
	Details struct {
		VenueID      string       `json:"venue_id"`
		DeliveryInfo DeliveryInfo `json:"delivery_info"`
	} `json:"details"`
	HostID                   string        `json:"host_id"`
	Participants             []Participant `json:"participants"`
	ParsedDeliveryCoordinate Coordinate    `json:"-"`
	Host                     string        `json:"-"`
}

const (
	StatusActive       = "active"
	StatusCanceled     = "cancelled"
	StatusPendingTrans = "pending_transaction"
	StatusPurchased    = "purchased"
)

const DeliveryCoordinatesPath = "details.delivery_info.location.coordinates.coordinates"

func ParseOrderDetails(orderDetailsJSON []byte) (*OrderDetails, error) {
	o := &OrderDetails{}
	var err error

	err = json.Unmarshal(orderDetailsJSON, o)
	if err != nil {
		return nil, fmt.Errorf("unmarshal details: %w", err)
	}

	o.ParsedDeliveryCoordinate = Coordinate{
		Lat: o.Details.DeliveryInfo.Location.Coordinates.Coordinates[0],
		Lon: o.Details.DeliveryInfo.Location.Coordinates.Coordinates[1],
	}

	o.Host, err = o.host()
	if err != nil {
		return nil, fmt.Errorf("get host: %w", err)
	}

	return o, nil
}

func (o *OrderDetails) host() (string, error) {
	for _, participant := range o.Participants {
		if participant.UserID == o.HostID {
			return participant.Name(), nil
		}
	}

	return "", fmt.Errorf("user matching host ID %q not found", o.HostID)
}

func (o *OrderDetails) RateByPerson() (map[string]float64, error) {
	output := make(map[string]float64)
	for _, participant := range o.Participants {
		total := 0.0
		for _, item := range participant.Basket.Items {
			total += item.EndAmount / 100
		}
		if total == 0 {
			continue
		}

		output[participant.Name()] = total
	}

	return output, nil
}
