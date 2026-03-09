package engine

// PacketVerdict tells the packet source what to do with the original packet.
type PacketVerdict int

const (
	// VerdictAccept forwards the packet unchanged.
	VerdictAccept PacketVerdict = iota
	// VerdictDrop suppresses the original packet (modified copies are already sent by the handler).
	VerdictDrop
)

// Engine is the interface for a packet processing backend.
// Both NFQUEUE and TUN implement this interface.
type Engine interface {
	Start() error
	Stop()
}
