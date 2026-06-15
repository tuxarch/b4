package nfq

import (
	"github.com/daniellavrushin/b4/engine"
	"github.com/daniellavrushin/b4/log"
	"github.com/daniellavrushin/b4/sock"
	"github.com/florianl/go-nfqueue"
)

type verdictCtx struct {
	id      uint32
	q       *nfqueue.Nfqueue
	verdict engine.PacketVerdict
}

func (vc *verdictCtx) accept() int {
	vc.verdict = engine.VerdictAccept
	if vc.q != nil {
		if err := vc.q.SetVerdict(vc.id, nfqueue.NfAccept); err != nil {
			log.Tracef("failed to set verdict on packet %d: %v", vc.id, err)
		}
	}
	return 0
}

func (vc *verdictCtx) drop() bool {
	vc.verdict = engine.VerdictDrop
	if vc.q != nil {
		if err := vc.q.SetVerdict(vc.id, nfqueue.NfDrop); err != nil {
			log.Tracef("failed to set drop verdict on packet %d: %v", vc.id, err)
			return false
		}
	}
	return true
}

func (w *Worker) InitSender() error {
	if w.sock != nil {
		return nil
	}
	cfg := w.getConfig()
	device := ""
	if cfg.Queue.Mode == "tun" {
		device = cfg.Queue.TUN.OutInterface
	}
	s, err := sock.NewSenderWithMarkDevice(int(cfg.Queue.Mark), device)
	if err != nil {
		return err
	}
	w.sock = s
	return nil
}

func (w *Worker) ProcessPacket(raw []byte) engine.PacketVerdict {
	if len(raw) == 0 {
		return engine.VerdictAccept
	}
	vc := &verdictCtx{verdict: engine.VerdictAccept}
	w.dispatch(vc, raw)
	return vc.verdict
}
