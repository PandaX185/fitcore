package postgres

import (
	"context"

	"gorm.io/gorm"
)

// transactionContextKey scopes the transaction value to this package.
type transactionContextKey struct{}

var txKey = transactionContextKey{}

type TransactionManager struct {
	conn *gorm.DB
}

func NewTransactionManager(conn *gorm.DB) *TransactionManager {
	return &TransactionManager{conn: conn}
}

func (m *TransactionManager) WithinTransaction(ctx context.Context, fn func(context.Context) error) error {
	return m.conn.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		return fn(context.WithValue(ctx, txKey, tx))
	})
}

func FromContext(ctx context.Context, base *gorm.DB) *gorm.DB {
	if tx, ok := ctx.Value(txKey).(*gorm.DB); ok {
		return tx
	}
	return base
}
