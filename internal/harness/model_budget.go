package harness

import (
	"context"
	"sync"
)

type modelBudgetKey struct{}
type modelReservation func(context.Context) error

// ReserveModelRequest must precede each actual model call, including calls
// nested inside a capability. Outside a harness, the caller's quota applies.
func ReserveModelRequest(ctx context.Context) error {
	reserve, ok := ctx.Value(modelBudgetKey{}).(modelReservation)
	if !ok {
		return nil
	}
	return reserve(ctx)
}

func (e *Engine) withModelBudget(ctx context.Context, run *Run, maximum int) context.Context {
	var mu sync.Mutex
	reserve := modelReservation(func(ctx context.Context) error {
		mu.Lock()
		defer mu.Unlock()
		if err := ctx.Err(); err != nil {
			return err
		}
		if run.CancelRequested {
			return ErrCancelled
		}
		if run.Loop.ModelRequests >= maximum {
			return ErrBudgetExhausted
		}
		run.Loop.ModelRequests++
		run.Version++
		if err := e.saveRun(ctx, run); err != nil {
			return err
		}
		if run.CancelRequested {
			return ErrCancelled
		}
		return nil
	})
	return context.WithValue(ctx, modelBudgetKey{}, reserve)
}
