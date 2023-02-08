package book_archive

import (
	"context"
	"github.com/rs/zerolog"
	"golang.org/x/sync/errgroup"
)

type AppContext struct {
	Logger      zerolog.Logger
	Ctx         context.Context
	CtxCancelFn context.CancelFunc
	Errors      error          // to collect many error if it has it
	ErrGroup    errgroup.Group // to sync loops
}

func NewAppContext(logger zerolog.Logger, ctx context.Context, ctxCancelFn context.CancelFunc) *AppContext {
	return &AppContext{Logger: logger, Ctx: ctx, CtxCancelFn: ctxCancelFn}
}
