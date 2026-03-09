package tun

import (
	"os"
	"sync"

	"github.com/daniellavrushin/b4/config"
	"github.com/daniellavrushin/b4/engine"
	"github.com/daniellavrushin/b4/log"
	"github.com/daniellavrushin/b4/nfq"
	"github.com/daniellavrushin/b4/sock"
)

const tunBufSize = 65536

// Engine implements the TUN-based packet processing backend.
// It creates a TUN device, routes traffic through it, and processes
// packets using the same Worker.ProcessPacket logic as NFQUEUE mode.
type Engine struct {
	cfg     *config.Config
	pool    *nfq.Pool
	tunFile *os.File
	tunName string
	routes  *routeManager
	sender  *sock.Sender
	wg      sync.WaitGroup
	quit    chan struct{}
}

// NewEngine creates a new TUN engine. It reuses the nfq.Pool for packet
// processing workers and the same matching/evasion logic.
func NewEngine(cfg *config.Config, pool *nfq.Pool) *Engine {
	return &Engine{
		cfg:  cfg,
		pool: pool,
		quit: make(chan struct{}),
	}
}

// Start opens the TUN device, sets up routing, and starts read loops.
func (e *Engine) Start() error {
	tunCfg := &e.cfg.Queue.TUN

	// Initialize sender for each worker (without opening NFQUEUE)
	for _, w := range e.pool.Workers {
		if err := w.InitSender(); err != nil {
			return err
		}
	}

	// Clean up stale TUN device from a previous unclean shutdown
	run("ip", "link", "del", tunCfg.DeviceName)

	// Open TUN device
	f, name, err := openTUN(tunCfg.DeviceName)
	if err != nil {
		return err
	}
	e.tunFile = f
	e.tunName = name
	log.Infof("TUN: opened device %s", name)

	// Create a sender for forwarding unmatched packets
	sender, err := sock.NewSenderWithMark(int(e.cfg.Queue.Mark))
	if err != nil {
		e.tunFile.Close()
		return err
	}
	e.sender = sender

	// Setup routing
	e.routes = newRouteManager(
		name,
		tunCfg.Address,
		tunCfg.OutInterface,
		tunCfg.OutGateway,
		e.cfg.Queue.Mark,
		tunCfg.RouteTable,
	)
	if err := e.routes.setup(); err != nil {
		e.sender.Close()
		e.tunFile.Close()
		return err
	}

	// Start reader goroutines (one per worker thread for parallelism)
	threads := e.cfg.Queue.Threads
	if threads < 1 {
		threads = 1
	}
	for i := 0; i < threads; i++ {
		e.wg.Add(1)
		go e.readLoop(i)
	}

	log.Infof("TUN: started %d reader threads", threads)
	return nil
}

// readLoop reads packets from the TUN device and processes them.
func (e *Engine) readLoop(workerIdx int) {
	defer e.wg.Done()

	worker := e.pool.Workers[workerIdx%len(e.pool.Workers)]
	buf := make([]byte, tunBufSize)

	for {
		select {
		case <-e.quit:
			return
		default:
		}

		n, err := e.tunFile.Read(buf)
		if err != nil {
			select {
			case <-e.quit:
				return
			default:
			}
			log.Errorf("TUN: read error: %v", err)
			continue
		}

		if n == 0 {
			continue
		}

		// Make a copy since buf is reused
		raw := make([]byte, n)
		copy(raw, buf[:n])

		verdict := worker.ProcessPacket(raw)

		if verdict == engine.VerdictAccept {
			// Forward the packet unchanged via raw socket (marked, bypasses TUN)
			e.forwardPacket(raw)
		}
		// VerdictDrop: ProcessPacket already sent modified copies via raw socket
	}
}

// forwardPacket sends an unmodified packet out via the real interface.
func (e *Engine) forwardPacket(raw []byte) {
	if len(raw) == 0 {
		return
	}
	v := raw[0] >> 4
	switch v {
	case 4:
		if len(raw) < 20 {
			return
		}
		dst := raw[16:20]
		_ = e.sender.SendIPv4(raw, dst)
	case 6:
		if len(raw) < 40 {
			return
		}
		dst := raw[24:40]
		_ = e.sender.SendIPv6(raw, dst)
	}
}

// Stop tears down routing and closes the TUN device.
func (e *Engine) Stop() {
	close(e.quit)

	// Close TUN fd to unblock readers
	if e.tunFile != nil {
		e.tunFile.Close()
	}

	e.wg.Wait()

	if e.routes != nil {
		e.routes.teardown()
	}
	if e.sender != nil {
		e.sender.Close()
	}

	log.Infof("TUN: engine stopped")
}
