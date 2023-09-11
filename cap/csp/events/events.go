package events

import (
	"context"

	api "github.com/wetware/pkg/api/process"
)

type EventHandler struct {
	pause  chan struct{}
	resume chan struct{}
	stop   chan struct{}
}

func (e EventHandler) Pause(ctx context.Context, call api.Events_pause) error {
	return chanOrCtx(ctx, e.pause)
}

func (e EventHandler) Resume(ctx context.Context, call api.Events_resume) error {
	return chanOrCtx(ctx, e.resume)
}

func (e EventHandler) Stop(ctx context.Context, call api.Events_stop) error {
	return chanOrCtx(ctx, e.stop)
}

func chanOrCtx(ctx context.Context, c chan struct{}) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	case c <- struct{}{}:
	}
	return nil
}

func (e EventHandler) OnPause() <-chan struct{} {
	return e.pause
}

func (e EventHandler) OnResume() <-chan struct{} {
	return e.pause
}

func (e EventHandler) OnStop() <-chan struct{} {
	return e.pause
}
