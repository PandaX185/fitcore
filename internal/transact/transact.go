package transact

import "context"

// Transactor runs fn inside a single transaction, committing on nil error
// and rolling back otherwise. Implementations are provided by the
// persistence edge (postgres.TransactionManager satisfies this interface
// structurally); domain services depend only on this port so the modules
// stay free of infrastructure imports.
type Transactor interface {
	WithinTransaction(ctx context.Context, fn func(context.Context) error) error
}
