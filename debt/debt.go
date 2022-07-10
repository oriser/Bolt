package debt

import (
	"time"

	"github.com/google/uuid"
)

type Debt struct {
	ID                   string    `db:"id"`
	BorrowerID           string    `db:"borrower_id"`
	LenderID             string    `db:"lender_id"`
	OrderID              string    `db:"order_id"`
	Amount               float64   `db:"amount"`
	InitiatedTransportID string    `db:"initial_transport"`
	MessageID            string    `db:"thread_ts"`
	CreatedAt            time.Time `db:"created_at"`
}

type Store interface {
	AdDebt(debt *Debt) error
	RemoveDebtInOrderID(orderID, debtID string) error
	ListDebtsForOrderID(orderID string) ([]*Debt, error)
}

func NewDebt(borrowerID, lenderID, orderID, initiatedTransportID, messageID string, amount float64) *Debt {
	return &Debt{
		ID:                   uuid.NewString(),
		BorrowerID:           borrowerID,
		LenderID:             lenderID,
		OrderID:              orderID,
		Amount:               amount,
		InitiatedTransportID: initiatedTransportID,
		MessageID:            messageID,
		CreatedAt:            time.Now(),
	}
}
