package online

import (
	"context"
	"sync"
)

type onlinekey int

var (
	waitKey = onlinekey(1)
)

func WithOnline(ctx context.Context, waiters int) context.Context {
	group := new(sync.WaitGroup)
	group.Add(waiters)
	return context.WithValue(ctx, waitKey, group)
}

func Online(ctx context.Context) {
	if g, ok := ctx.Value(waitKey).(*sync.WaitGroup); ok {
		g.Done()
	}
}

func Wait(ctx context.Context) {
	if g, ok := ctx.Value(waitKey).(*sync.WaitGroup); ok {
		g.Wait()
	}
}
