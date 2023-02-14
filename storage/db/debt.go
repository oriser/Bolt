package db

import (
	_ "embed"
	"fmt"
	"time"

	sq "github.com/Masterminds/squirrel"
	"github.com/google/uuid"
	"github.com/oriser/bolt/debt"
)

func (d *DBStore) AddDebt(debt *debt.Debt) error {
	if debt == nil {
		return fmt.Errorf("nil debt")
	}
	if debt.ID == "" {
		debt.ID = uuid.NewString()
	}
	debt.CreatedAt = time.Now()

	sql, args, err := sq.Insert("debts").Values(debt.ID, debt.BorrowerID, debt.LenderID, debt.OrderID,
		debt.Amount, debt.InitiatedTransportID, debt.MessageID, debt.CreatedAt).ToSql()
	if err != nil {
		return fmt.Errorf("generating insert SQL: %w", err)
	}

	if _, err = d.db.Exec(sql, args...); err != nil {
		return newExecError("adding debt", sql, err, args...)
	}
	return nil
}

func (d *DBStore) RemoveDebtInOrderID(orderID, debtID string) error {
	sql, args, err := sq.Delete("debts").Where("order_id=? AND id=?", orderID, debtID).ToSql()
	if err != nil {
		return fmt.Errorf("generating delete SQL: %w", err)
	}

	if _, err = d.db.Exec(sql, args...); err != nil {
		return newExecError("deleting debt", sql, err, args...)
	}

	return nil
}

func (d *DBStore) ListDebtsForOrderID(orderID string) ([]*debt.Debt, error) {
	sql, args, err := sq.Select("*").From("debts").Where("order_id=?", orderID).ToSql()
	if err != nil {
		return nil, fmt.Errorf("generating delete SQL: %w", err)
	}

	debts := []*debt.Debt{}
	err = d.db.Select(&debts, sql, args...)
	if err != nil {
		return nil, newExecError("selecting debts", sql, err, args...)
	}

	return debts, nil
}
