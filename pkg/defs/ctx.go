package defs

import "context"

type UserCtx struct {
	context.Context
	context.CancelFunc
	Close func(msg ...interface{})
}
