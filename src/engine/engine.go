package engine

type PacketVerdict int

const (
	VerdictAccept PacketVerdict = iota
	VerdictDrop
)

type Engine interface {
	Start() error
	Stop()
}
