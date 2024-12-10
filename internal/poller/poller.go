package poller

import (
	"context"
	"time"

	"github.com/rs/zerolog/log"

	"github.com/babylonlabs-io/staking-expiry-checker/internal/config"
	"github.com/babylonlabs-io/staking-expiry-checker/internal/types"
)

type PollerType string

const (
	ExpiryPoller        PollerType = "expiry"
	BTCSubscriberPoller PollerType = "btc-subscriber"
)

type PollerOperation func(ctx context.Context) *types.Error

type Poller struct {
	pollerType PollerType
	operation  PollerOperation
	interval   time.Duration
	timeout    time.Duration
	quit       chan struct{}
}

func NewPoller(pollerType PollerType, cfg config.PollerConfig, operation PollerOperation) (*Poller, error) {
	return &Poller{
		pollerType: pollerType,
		operation:  operation,
		interval:   cfg.Interval,
		timeout:    cfg.Timeout,
		quit:       make(chan struct{}),
	}, nil
}

func (p *Poller) Start(ctx context.Context) {
	ticker := time.NewTicker(p.interval)

	for {
		select {
		case <-ticker.C:
			// Start a new context for each poll
			pollingCtx, cancel := context.WithTimeout(ctx, p.timeout)
			defer cancel()

			if err := p.operation(pollingCtx); err != nil {
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

// func (p *Poller) poll(ctx context.Context) error {
// 	log.Debug().Msg("Polling started")
// 	if err := p.service.ProcessExpiredDelegations(ctx); err != nil {
// 		log.Error().Err(err).Msg("Error processing expired delegations")
// 		return err
// 	}
// 	log.Debug().Msg("Polling completed")
// 	return nil
// }
