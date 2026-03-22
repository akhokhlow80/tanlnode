package tx

import (
	"context"
	"fmt"
	"log"
	"runtime/debug"
)

type RollbackError struct {
	rollbackErr error
	cause       error
}

func (err *RollbackError) Unwrap() []error {
	return []error{err.rollbackErr, err.cause}
}

func (err *RollbackError) Error() string {
	return fmt.Sprintf("rollback error: %s, caused by: %s", err.rollbackErr, err.cause)
}

type Transactional struct {
	// set nil to do nothing
	Commit func(ctx context.Context) error
	Action func(ctx context.Context) error
	// set nil to do nothing
	Rollback func(ctx context.Context) error
}

func (t Transactional) Do(ctx context.Context) error {
	err := func() error {
		if t.Rollback != nil {
			defer func() {
				if r := recover(); r != nil {
					debug.PrintStack()
					if err := t.Rollback(ctx); err != nil {
						log.Printf("Error rolling back after panic: %s", err)
					} else {
						log.Printf("Rolled back after panic")
					}
					panic(r)
				}
			}()
		}

		return t.Action(ctx)
	}()

	if err != nil {
		if t.Rollback == nil {
			return err
		}
		rbErr := t.Rollback(ctx)
		if rbErr != nil {
			return &RollbackError{rbErr, err}
		} else {
			return err
		}
	}
	if t.Commit == nil {
		return nil
	}
	return t.Commit(ctx)
}
