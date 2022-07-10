package wolt

import (
	"fmt"
	"log"
	"strings"

	"github.com/Jeffail/gabs/v2"
)

type OrderHistory struct {
	parsedOutput *gabs.Container
}

func (o *OrderHistory) GetOrderJsonByID(orderPrettyID string) (*gabs.Container, bool) {
	for _, order := range o.parsedOutput.Children() {
		if !order.Exists("group", "url") {
			continue
		}
		if strings.HasSuffix(order.S("group", "url").Data().(string), orderPrettyID) {
			return order, true
		}
	}
	return nil, false
}

func (o *OrderHistory) DeliveryRateForOrder(orderPrettyID string) (int, error) {
	order, ok := o.GetOrderJsonByID(orderPrettyID)
	if !ok {
		return 0, fmt.Errorf("order %s not found in order history", orderPrettyID)
	}

	if !order.Exists("delivery_price") {
		return 0, fmt.Errorf("delivery_price not found in order")
	}

	price := int(order.S("delivery_price").Data().(float64)) / 100
	if price < 10 {
		log.Printf("Price is smaller then 10 (%d), set to 10", price)
		price = 10
	}

	if price > 20 {
		log.Printf("Price is larger then 20 (%d), set to 20", price)
		price = 20
	}
	return price, nil
}
