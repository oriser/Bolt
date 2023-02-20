package db

import (
	"bytes"
	_ "embed"
	"testing"
	"text/template"
	"time"

	"github.com/Masterminds/sprig"
	"github.com/google/uuid"
	debtDomain "github.com/oriser/bolt/debt"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

//go:embed debt_test_seed.sql
var seed string

type debtBuilder struct {
	debt *debtDomain.Debt
}

func (d *debtBuilder) WithOrderID(orderID string) *debtBuilder {
	d.debt.OrderID = orderID
	return d
}
func (d *debtBuilder) Debt() *debtDomain.Debt {
	return d.debt
}

func getDummyDebt() *debtBuilder {
	return &debtBuilder{&debtDomain.Debt{
		BorrowerID:           "borrower_" + uuid.NewString(),
		LenderID:             "lender_" + uuid.NewString(),
		OrderID:              "order_" + uuid.NewString(),
		Amount:               10,
		InitiatedTransportID: "transport_" + uuid.NewString(),
		MessageID:            "threadTs_" + uuid.NewString(),
	}}
}

// For the equal to work, dumping and re-parsing the time object the get rid of unimportant changes.
func formatTime(t *testing.T, src time.Time) time.Time {
	str := src.Format(time.RFC3339)
	got, err := time.Parse(time.RFC3339, str)
	require.NoError(t, err)
	return got
}

func TestAddListForOrderAndRemove(t *testing.T) {
	t.Parallel()

	dbTest := NewDBTest(t)
	t.Cleanup(func() {
		dbTest.Cleanup(t)
	})

	seedTemplate := template.Must(template.New("seed").Funcs(sprig.TxtFuncMap()).Parse(seed))
	rawSeedSQL := bytes.NewBuffer(nil)
	require.NoError(t, seedTemplate.Execute(rawSeedSQL, nil))

	_, err := dbTest.db.db.Exec(rawSeedSQL.String())
	require.NoError(t, err)

	tests := []struct {
		name  string
		debts []*debtDomain.Debt
	}{
		{
			name:  "Happy flow",
			debts: []*debtDomain.Debt{getDummyDebt().Debt()},
		},
		{
			name: "Multiple debts different orders",
			debts: []*debtDomain.Debt{
				getDummyDebt().Debt(),
				getDummyDebt().Debt(),
				getDummyDebt().Debt(),
				getDummyDebt().Debt(),
			},
		},
		{
			name: "Multiple debts dame order",
			debts: []*debtDomain.Debt{
				getDummyDebt().WithOrderID("order").Debt(),
				getDummyDebt().WithOrderID("order").Debt(),
				getDummyDebt().WithOrderID("order").Debt(),
				getDummyDebt().WithOrderID("order").Debt(),
				getDummyDebt().WithOrderID("order").Debt(),
				getDummyDebt().WithOrderID("order").Debt(),
			},
		},
		{
			name: "Multiple debts multiple orders",
			debts: []*debtDomain.Debt{
				getDummyDebt().WithOrderID("order1").Debt(),
				getDummyDebt().WithOrderID("order1").Debt(),
				getDummyDebt().WithOrderID("order2").Debt(),
				getDummyDebt().WithOrderID("order2").Debt(),
				getDummyDebt().WithOrderID("order2").Debt(),
				getDummyDebt().WithOrderID("order1").Debt(),
			},
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			expectedOrderIDs := make(map[string][]debtDomain.Debt)

			// Adding debts
			for _, debt := range tc.debts {
				currentOrderID := debt.OrderID

				err := dbTest.db.AddDebt(debt)
				assert.NoError(t, err)

				assert.NotEmpty(t, debt.ID)
				assert.NotEmpty(t, debt.CreatedAt)
				assert.Greater(t, int(debt.CreatedAt.Sub(time.Now().Add(-1*time.Minute))), 0)

				expectedOrderIDs[currentOrderID] = append(expectedOrderIDs[currentOrderID], *debt)
			}

			// Going over per-order debts
			for orderID, expectedDebts := range expectedOrderIDs {
				debts, err := dbTest.db.ListDebtsForOrderID(orderID)
				assert.NoError(t, err)
				assert.Len(t, debts, len(expectedDebts))

				derefDebts := make([]debtDomain.Debt, len(debts))
				for i, debt := range debts {
					require.NotNil(t, debt)
					derefDebts[i] = *debt
				}

				// Checking that per-order debts are as expected
				for _, expectedDebt := range expectedDebts {
					found := false
					for i, debt := range derefDebts {
						require.NotNil(t, debt)
						assert.Equal(t, debt.OrderID, orderID)

						expectedDebt.CreatedAt = formatTime(t, expectedDebt.CreatedAt)
						debt.CreatedAt = formatTime(t, debt.CreatedAt)
						if assert.ObjectsAreEqual(expectedDebt, debt) {
							found = true
							derefDebts = append(derefDebts[:i], derefDebts[i+1:]...)
							break
						}
					}
					assert.True(t, found, "Expected to see debt in order ID %q: %#v\nFound debts:%#v", orderID, expectedDebt, derefDebts)
				}

				// Removing debts for current order ID
				for _, debt := range debts {
					err = dbTest.db.RemoveDebtInOrderID(orderID, debt.ID)
					assert.NoError(t, err)
				}

				// Checking that indeed it deleted
				debts, err = dbTest.db.ListDebtsForOrderID(orderID)
				assert.NoError(t, err)
				assert.Len(t, debts, 0)
			}
		})
	}
}
