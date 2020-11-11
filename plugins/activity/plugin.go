package activity

import (
	"context"

	"github.com/spiral/errors"
	"github.com/spiral/roadrunner/v2/log"
	"github.com/spiral/roadrunner/v2/plugins/app"
	"github.com/spiral/roadrunner/v2/util"
	"github.com/temporalio/roadrunner-temporal/plugins/temporal"
	"go.uber.org/zap"
)

const (
	// PluginName defines public service name.
	PluginName = "activities"

	// RRMode sets as RR_MODE env variable to let worker know about the mode to run.
	RRMode = "temporal/activity"
)

// Plugin to manage activity execution.
type Plugin struct {
	temporal temporal.Temporal
	events   *util.EventHandler
	app      app.WorkerFactory
	log      log.Logger
	pool     *activityPool
}

// Init configures activity service.
func (svc *Plugin) Init(temporal temporal.Temporal, app app.WorkerFactory, log log.Logger) error {
	if temporal.GetConfig().Activities == nil {
		// no need to serve activities
		return errors.E(errors.Disabled)
	}

	svc.temporal = temporal
	svc.app = app
	svc.log = log

	return nil
}

// Serve activities with underlying workers.
func (svc *Plugin) Serve() chan error {
	errCh := make(chan error, 1)

	pool, err := NewActivityPool(context.Background(), *svc.temporal.GetConfig().Activities, svc.app)
	if err != nil {
		errCh <- errors.E(errors.Op("newActivityPool"), err)
		return errCh
	}

	// todo: proxy events

	err = pool.Start(context.Background(), svc.temporal)
	if err != nil {
		errCh <- errors.E(errors.Op("startActivityPool"), err)
		return errCh
	}

	svc.pool = pool

	svc.log.Debug("Started activity processing", zap.Any("activities", pool.activities))

	return errCh
}

func (svc *Plugin) Stop() error {
	if svc.pool != nil {
		svc.pool.Destroy(context.Background())
	}
	return nil
}

// Name of the service.
func (svc *Plugin) Name() string {
	return PluginName
}

// Reset resets underlying workflow pool with new copy.
func (svc *Plugin) Reset() error {
	svc.log.Debug("Reset activity worker pool")

	pool, err := NewActivityPool(context.Background(), *svc.temporal.GetConfig().Activities, svc.app)
	if err != nil {
		return errors.E(errors.Op("newActivityPool"), err)
	}

	// todo: proxy events
	err = pool.Start(context.Background(), svc.temporal)
	if err != nil {
		return errors.E(errors.Op("startActivityPool"), err)
	}

	previous := svc.pool
	svc.pool = pool

	previous.Destroy(context.Background())

	return nil
}

// AddListener adds event listeners to the service.
func (svc *Plugin) AddListener(listener util.EventListener) {
	svc.events.AddListener(listener)
}
