package engine

const ReinjectMarkBit = 0x800000

type PacketVerdict int

const (
	VerdictAccept PacketVerdict = iota
	VerdictDrop
)

type Engine interface {
	Start() error
	Stop()
}
