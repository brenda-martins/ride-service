package app

import "context"

// withinTx runs fn inside a transaction when a manager is configured,
// otherwise it runs fn directly.
func withinTx(ctx context.Context, tx TxManager, fn func(ctx context.Context) error) error {
	if tx == nil {
		return fn(ctx)
	}
	return tx.WithinTx(ctx, fn)
}
