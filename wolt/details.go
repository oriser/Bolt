package wolt

import (
	"fmt"

	"github.com/Jeffail/gabs/v2"
)

type OrderDetails struct {
	ParsedOutput *gabs.Container
}

const (
	StatusActive       = "active"
	StatusCanceled     = "cancelled" // nolint
	StatusPendingTrans = "pending_transaction"
	StatusPurchased    = "purchased"
)

const DeliveryCoordinatesPath = "details.delivery_info.location.coordinates.coordinates"

func (o *OrderDetails) Status() (string, error) {
	if !o.ParsedOutput.Exists("status") {
		return "", fmt.Errorf("'status' key not found in output json")
	}

	return o.ParsedOutput.S("status").Data().(string), nil
}

func (o *OrderDetails) VenueID() (string, error) {
	if !o.ParsedOutput.Exists("details", "venue_id") {
		return "", fmt.Errorf("'details.venue_id' key not found in output json")
	}
	return o.ParsedOutput.S("details", "venue_id").Data().(string), nil
}

func (o *OrderDetails) DeliveryCoordinate() (Coordinate, error) {
	if !o.ParsedOutput.ExistsP(DeliveryCoordinatesPath) {
		return Coordinate{}, fmt.Errorf("%q key not found in output json", DeliveryCoordinatesPath)
	}

	return CoordinateFromArray(o.ParsedOutput.Path(DeliveryCoordinatesPath))
}

func (o *OrderDetails) Host() (string, error) {
	if !o.ParsedOutput.Exists("participants") {
		return "", fmt.Errorf("no participants")
	}
	if !o.ParsedOutput.Exists("host_id") {
		return "", fmt.Errorf("no host_id")
	}

	hostID := o.ParsedOutput.S("host_id").Data().(string)
	if hostID == "" {
		return "", fmt.Errorf("empty host_id")
	}

	for _, participant := range o.ParsedOutput.S("participants").Children() {
		if participant.S("user_id").Data().(string) == hostID {
			return o.nameFromParticipant(participant), nil
		}
	}

	return "", fmt.Errorf("user matching host ID %q not found", hostID)
}

func (o *OrderDetails) RateByPerson() (map[string]float64, error) {
	if !o.ParsedOutput.Exists("participants") {
		return nil, fmt.Errorf("no participants")
	}

	output := make(map[string]float64)
	for _, participant := range o.ParsedOutput.S("participants").Children() {
		if !participant.Exists("basket", "items") {
			continue
		}
		total := 0.0
		for _, item := range participant.S("basket", "items").Children() {
			if !item.Exists("end_amount") {
				continue
			}
			total += item.S("end_amount").Data().(float64) / 100
		}
		if total == 0 {
			continue
		}

		output[o.nameFromParticipant(participant)] = total
	}

	return output, nil
}

func (o *OrderDetails) nameFromParticipant(participant *gabs.Container) string {
	name := participant.S("first_name").Data().(string)
	lastName := ""
	if participant.Exists("last_name") {
		lastNameInt := participant.S("last_name").Data()
		if lastNameInt != nil {
			lastName = lastNameInt.(string)
		}
	}
	if lastName != "" {
		name = name + " " + lastName
	}
	return name
}
