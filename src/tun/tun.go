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

const (
	tunBufSize        = 65536
	defaultDeviceName = "b4tun0"
	defaultAddress    = "10.255.0.1/30"
	defaultRouteTable = 9999
)

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

func NewEngine(cfg *config.Config, pool *nfq.Pool) *Engine {
	return &Engine{
		cfg:  cfg,
		pool: pool,
		quit: make(chan struct{}),
	}
}

func (e *Engine) Start() error {
	tunCfg := &e.cfg.Queue.TUN
	deviceName := tunCfg.DeviceName
	if deviceName == "" {
		deviceName = defaultDeviceName
	}
	address := tunCfg.Address
	if address == "" {
		address = defaultAddress
	}
	routeTable := tunCfg.RouteTable
	if routeTable == 0 {
		routeTable = defaultRouteTable
	}

	for _, w := range e.pool.Workers {
		if err := w.InitSender(); err != nil {
			return err
		}
	}

	run("ip", "link", "del", deviceName)

	f, name, err := openTUN(deviceName)
	if err != nil {
		return err
	}
	e.tunFile = f
	e.tunName = name
	log.Infof("TUN: opened device %s", name)

	sender, err := sock.NewSenderWithMarkDevice(int(e.cfg.Queue.Mark), tunCfg.OutInterface)
	if err != nil {
		e.tunFile.Close()
		return err
	}
	e.sender = sender

	e.routes = &routeManager{
		tunName:    name,
		tunAddr:    address,
		tunAddrV6:  tunCfg.AddressV6,
		outIface:   tunCfg.OutInterface,
		outGateway: tunCfg.OutGateway,
		mark:       e.cfg.Queue.Mark,
		routeTable: routeTable,
		routes:     tunCfg.Routes,
	}
	if err := e.routes.setup(); err != nil {
		e.sender.Close()
		e.tunFile.Close()
		return err
	}

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

		raw := make([]byte, n)
		copy(raw, buf[:n])

		if worker.ProcessPacket(raw) == engine.VerdictAccept {
			e.forwardPacket(raw)
		}
	}
}

func (e *Engine) forwardPacket(raw []byte) {
	if len(raw) == 0 {
		return
	}
	switch raw[0] >> 4 {
	case 4:
		if len(raw) < 20 {
			return
		}
		_ = e.sender.SendIPv4(raw, raw[16:20])
	case 6:
		if len(raw) < 40 {
			return
		}
		_ = e.sender.SendIPv6(raw, raw[24:40])
	}
}

func (e *Engine) Stop() {
	close(e.quit)

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
