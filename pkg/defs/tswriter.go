package defs

type TsWriter interface {
	Write(b []byte, ts int64) (i int, err error)
}
