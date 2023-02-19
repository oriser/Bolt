package db

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	sq "github.com/Masterminds/squirrel"
	"github.com/google/uuid"
	"github.com/oriser/bolt/order"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func getDummyOrder() *order.Order {
	return &order.Order{
		ID:         uuid.NewString(),
		OriginalID: "ABCD",
		CreatedAt:  time.Now(),
		Receiver:   "receiver",
		VenueName:  "testven",
		VenueID:    "testvenid",
		VenueLink:  "venlink",
		VenueCity:  "vencity",
		Host:       "host",
		HostID:     "hostid",
		Participants: []order.Participant{
			{
				Name:   "Test 1",
				Amount: 50.4,
			},
			{
				Name:   "Test2",
				ID:     "id123",
				Amount: 20.431,
			},
		},
		Status:       order.StatusDone,
		DeliveryRate: 50,
	}
}

func TestSaveOrder(t *testing.T) {
	t.Parallel()

	dbTest := NewDBTest(t)
	t.Cleanup(func() {
		dbTest.Cleanup(t)
	})

	tests := []struct {
		name  string
		count int
	}{
		{
			name:  "one order",
			count: 1,
		},
		{
			name:  "multiple orders",
			count: 3,
		},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			for i := 0; i < tc.count; i++ {
				savedOrder := getDummyOrder()
				err := dbTest.db.SaveOrder(context.Background(), savedOrder)
				require.NoError(t, err)

				sql, args, err := sq.Select("*").From("orders").Where("id=?", savedOrder.ID).ToSql()
				require.NoError(t, err)
				var listedOrders []*orderModel
				err = dbTest.db.db.Select(&listedOrders, sql, args...)
				require.NoError(t, err)

				require.Len(t, listedOrders, 1)

				err = json.Unmarshal(listedOrders[0].MarshaledParticipants, &listedOrders[0].Order.Participants)
				require.NoError(t, err)
				listedOrders[0].CreatedAt = formatTime(t, listedOrders[0].CreatedAt)
				savedOrder.CreatedAt = formatTime(t, listedOrders[0].CreatedAt)
				assert.Equal(t, savedOrder, listedOrders[0].Order)
			}
		})
	}
}
