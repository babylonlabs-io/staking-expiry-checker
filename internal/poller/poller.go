package poller

import (
	"context"
	"time"

	"github.com/rs/zerolog/log"

	"github.com/babylonlabs-io/staking-expiry-checker/internal/config"
	"github.com/babylonlabs-io/staking-expiry-checker/internal/types"
)

type ExpiryChecker interface {
	ProcessExpiredDelegations(ctx context.Context) *types.Error
}

type BTCSubscriber interface {
	ProcessBTCSubscriber(ctx context.Context) *types.Error
}

type PollerType string

const (
	ExpiryPoller        PollerType = "expiry"
	BTCSubscriberPoller PollerType = "btc-subscriber"
)

type Poller struct {
	pollerType PollerType
	poll       func(ctx context.Context) *types.Error
	interval   time.Duration
	timeout    time.Duration
	quit       chan struct{}
}

func NewExpiryPoller(cfg config.PollerConfig, checker ExpiryChecker) *Poller {
	return &Poller{
		pollerType: ExpiryPoller,
		interval:   cfg.Interval,
		timeout:    cfg.Timeout,
		poll:       checker.ProcessExpiredDelegations,
		quit:       make(chan struct{}),
	}
}

func NewBTCSubscriberPoller(cfg config.PollerConfig, subscriber BTCSubscriber) *Poller {
	return &Poller{
		pollerType: BTCSubscriberPoller,
		interval:   cfg.Interval,
		timeout:    cfg.Timeout,
		poll:       subscriber.ProcessBTCSubscriber,
		quit:       make(chan struct{}),
	}
}

func (p *Poller) Start(ctx context.Context) {
	ticker := time.NewTicker(p.interval)

	for {
		select {
		case <-ticker.C:
			// Start a new context for each poll
			pollingCtx, cancel := context.WithTimeout(ctx, p.timeout)
			defer cancel()

			if err := p.poll(pollingCtx); err != nil {
				log.Error().
					Err(err).
					Str("poller", string(p.pollerType)).
					Msg("Error in polling operation")
			}
		case <-ctx.Done():
			// Handle context cancellation.
			log.Info().Msg("Poller stopped due to context cancellation")
			return
		case <-p.quit:
			ticker.Stop() // Stop the ticker
			return
		}
	}
}

func (p *Poller) Stop() {
	close(p.quit)
}
