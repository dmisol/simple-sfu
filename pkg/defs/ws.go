package defs

const (
	ActPublish   = "pub" // up, down
	ActInvite    = "inv" // down; front should start "sub" with the same "id"
	ActSubscribe = "sub" // up, down
	ActDelete    = "del" // down
)

type WsPload struct {
	Action string `json:"action"`
	Id     int64  `json:"id"`
	Data   []byte `json:"data,omitempty"`
}
