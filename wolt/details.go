package wolt

import (
	"encoding/json"
	"fmt"
	"time"
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

type Status string
type DeliveryStatus string
type DeliveryStatusToTimeMap map[DeliveryStatus]time.Time

func (deliveryStatusLog *DeliveryStatusToTimeMap) UnmarshalJSON(bytes []byte) error {
	o := &([]struct {
		Datetime struct {
			DateUnix int64 `json:"$date"`
		} `json:"datetime"`
		Status DeliveryStatus `json:"status"`
	}{})

	err := json.Unmarshal(bytes, o)
	if err != nil {
		return fmt.Errorf("error unmarshalling DeliveryStatusToTimeMap %w", err)
	}

	res := make(DeliveryStatusToTimeMap)
	for _, entry := range *o {
		if entry.Datetime.DateUnix == 0 || entry.Status == "" {
			return fmt.Errorf("error parsing delivery status log entry %w", err)
		}

		currentEntryTime := time.UnixMilli(entry.Datetime.DateUnix)
		// Duplicate statuses shouldn't occur, but if they do, we take their latest timestamp
		if existingTime, exists := res[entry.Status]; !exists || currentEntryTime.After(existingTime) {
			res[entry.Status] = currentEntryTime
		}
	}
	*deliveryStatusLog = res

	return nil
}

func (s Status) Purchased() bool {
	return s == StatusPurchased || s == StatusPendingTrans
}

type OrderDetails struct {
	Status        Status `json:"status"`
	CreatedAtUnix struct {
		DateUnix int64 `json:"$date"`
	} `json:"created_at"`
	Details struct {
		VenueID      string       `json:"venue_id"`
		DeliveryInfo DeliveryInfo `json:"delivery_info"`
	} `json:"details"`
	HostID       string        `json:"host_id"`
	Participants []Participant `json:"participants"`
	Purchase     struct {
		DeliveryEtaUnix struct {
			DateUnix int64 `json:"$date"`
		} `json:"delivery_eta"`
		DeliveryStatus       DeliveryStatus          `json:"delivery_status"`
		DeliveryStatusLog    DeliveryStatusToTimeMap `json:"delivery_status_log"`
		PurchaseDatetimeUnix struct {
			DateUnix int64 `json:"$date"`
		} `json:"purchase_datetime"`
	} `json:"purchase"`

	CreatedAt                time.Time  `json:"-"`
	DeliveryEta              time.Time  `json:"-"`
	PurchaseDatetime         time.Time  `json:"-"`
	ParsedDeliveryCoordinate Coordinate `json:"-"`
	Host                     string     `json:"-"`
}

const (
	StatusActive       Status = "active"
	StatusCanceled     Status = "cancelled"
	StatusPendingTrans Status = "pending_transaction"
	StatusPurchased    Status = "purchased"
)

const (
	DeliveryStatusDelivered DeliveryStatus = "delivered"
)

const DeliveryCoordinatesPath = "details.delivery_info.location.coordinates.coordinates"

func ParseOrderDetails(orderDetailsJSON []byte) (*OrderDetails, error) {
	o := &OrderDetails{}
	var err error

	err = json.Unmarshal(orderDetailsJSON, o)
	if err != nil {
		return nil, fmt.Errorf("unmarshal details: %w", err)
	}

	o.ParsedDeliveryCoordinate, err = CoordinateFromArray(o.Details.DeliveryInfo.Location.Coordinates.Coordinates)
	if err != nil {
		return nil, fmt.Errorf("parse coordinates: %w", err)
	}

	o.Host, err = o.host()
	if err != nil {
		return nil, fmt.Errorf("get host: %w", err)
	}

	o.CreatedAt = time.UnixMilli(o.CreatedAtUnix.DateUnix)
	o.DeliveryEta = time.UnixMilli(o.Purchase.DeliveryEtaUnix.DateUnix)
	o.PurchaseDatetime = time.UnixMilli(o.Purchase.PurchaseDatetimeUnix.DateUnix)

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

func (o *OrderDetails) IsDelivered() bool {
	return o.Purchase.DeliveryStatus == DeliveryStatusDelivered
}
