package activity

import (
	"context"
	"time"

	"github.com/cenkalti/backoff/v4"
	"github.com/spiral/roadrunner/v2/pkg/events"
	"github.com/spiral/roadrunner/v2/pkg/worker"
	"github.com/spiral/roadrunner/v2/plugins/config"
	roadrunner_temporal "github.com/temporalio/roadrunner-temporal"

	"sync"
	"sync/atomic"

	"github.com/spiral/errors"
	"github.com/spiral/roadrunner/v2/plugins/logger"
	"github.com/spiral/roadrunner/v2/plugins/server"
	"github.com/temporalio/roadrunner-temporal/client"
)

const (
	// PluginName defines public service name.
	PluginName = "activities"
)

// Plugin to manage activity execution.
type Plugin struct {
	temporal client.Temporal
	events   events.Handler
	server   server.Server
	log      logger.Logger
	mu       sync.Mutex
	reset    chan struct{}
	pool     activityPool
	// graceful timeout for the worker
	graceTimeout time.Duration
	closing      int64
}

// Init configures activity service.
func (p *Plugin) Init(temporal client.Temporal, server server.Server, log logger.Logger, cfg config.Configurer) error {
	const op = errors.Op("activity_plugin_init")
	if !cfg.Has(roadrunner_temporal.RootPluginName) {
		return errors.E(op, errors.Disabled)
	}

	if temporal.GetConfig().Activities == nil {
		// no need to serve activities
		return errors.E(op, errors.Disabled)
	}

	p.temporal = temporal
	p.server = server
	p.events = events.NewEventsHandler()
	p.log = log
	p.reset = make(chan struct{})

	// it can't be 0 (except set by user), because it would be set by the rr-binary (cli)
	p.graceTimeout = cfg.GetCommonConfig().GracefulTimeout

	return nil
}

// Serve activities with underlying workers.
func (p *Plugin) Serve() chan error {
	const op = errors.Op("activity_plugin_serve")

	errCh := make(chan error, 1)
	pool, err := p.startPool()
	if err != nil {
		errCh <- errors.E(op, err)
		return errCh
	}

	p.pool = pool

	go func() {
		// single case loop
		for range p.reset {
			if atomic.LoadInt64(&p.closing) == 1 {
				return
			}

			err := p.replacePool()
			if err == nil {
				continue
			}

			bkoff := backoff.NewExponentialBackOff()
			bkoff.InitialInterval = time.Second

			err = backoff.Retry(p.replacePool, bkoff)
			if err != nil {
				errCh <- errors.E(op, err)
			}
		}
	}()

	return errCh
}

// Stop stops the serving plugin.
func (p *Plugin) Stop() error {
	atomic.StoreInt64(&p.closing, 1)
	const op = errors.Op("activity_plugin_stop")

	pool := p.getPool()
	if pool != nil {
		p.pool = nil
		err := pool.Destroy(context.Background())
		if err != nil {
			return errors.E(op, err)
		}
		return nil
	}

	return nil
}

// Name of the service.
func (p *Plugin) Name() string {
	return PluginName
}

// RPC returns associated rpc service.
func (p *Plugin) RPC() interface{} {
	return &rpc{srv: p, client: p.temporal.GetClient()}
}

// Workers returns pool workers.
func (p *Plugin) Workers() []worker.BaseProcess {
	if p.getPool() == nil {
		return nil
	}
	return p.getPool().Workers()
}

// ActivityNames returns list of all available activities.
func (p *Plugin) ActivityNames() []string {
	return p.pool.ActivityNames()
}

// Reset resets underlying workflow pool with new copy.
func (p *Plugin) Reset() error {
	p.reset <- struct{}{}

	return nil
}

// AddListener adds event listeners to the service.
func (p *Plugin) AddListener(listener events.Listener) {
	p.events.AddListener(listener)
}

// AddListener adds event listeners to the service.
func (p *Plugin) poolListener(event interface{}) {
	if ev, ok := event.(events.PoolEvent); ok {
		if ev.Event == events.EventPoolError {
			p.log.Error("Activity pool error", "error", ev.Payload.(error))
			p.reset <- struct{}{}
		}
	}

	p.events.Push(event)
}

// AddListener adds event listeners to the service.
func (p *Plugin) startPool() (activityPool, error) {
	pool, err := newActivityPool(
		p.temporal.GetCodec().WithLogger(p.log),
		p.poolListener,
		*p.temporal.GetConfig().Activities,
		p.server,
	)

	if err != nil {
		return nil, errors.E(errors.Op("newActivityPool"), err)
	}

	err = pool.Start(context.Background(), p.temporal)
	if err != nil {
		return nil, errors.E(errors.Op("startActivityPool"), err)
	}

	p.log.Debug("Started activity processing", "activities", pool.ActivityNames())

	return pool, nil
}

func (p *Plugin) replacePool() error {
	const op = errors.Op("activity_plugin_replace_pool")
	pool, err := p.startPool()
	if err != nil {
		p.log.Error("replace activity pool failed", "error", err)
		return errors.E(op, err)
	}

	p.log.Debug("replace activity pool")

	var previous activityPool

	p.mu.Lock()
	previous, p.pool = p.pool, pool
	p.mu.Unlock()

	err = previous.Destroy(context.Background())
	if err != nil {
		p.log.Error(
			"Unable to destroy expired activity pool",
			"error",
			errors.E(op, err),
		)
	}

	return nil
}

// getPool returns currently pool.
func (p *Plugin) getPool() activityPool {
	p.mu.Lock()
	defer p.mu.Unlock()

	return p.pool
}
