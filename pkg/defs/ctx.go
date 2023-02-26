package defs

import "context"

type UserCtx struct {
	Id int64
	context.Context
	context.CancelFunc
	Close func(msg ...interface{})
}
