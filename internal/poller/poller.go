package poller

import (
	"context"
	"time"

	"github.com/rs/zerolog/log"

	"github.com/babylonlabs-io/staking-expiry-checker/internal/config"
	"github.com/babylonlabs-io/staking-expiry-checker/internal/services"
)

type Poller struct {
	service  *services.Service
	interval time.Duration
	timeout  time.Duration
	quit     chan struct{}
}

func NewPoller(cfg config.PollerConfig, service *services.Service) (*Poller, error) {
	return &Poller{
		service:  service,
		interval: cfg.Interval,
		timeout:  cfg.Timeout,
		quit:     make(chan struct{}),
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

			if err := p.poll(pollingCtx); err != nil {
				log.Error().Err(err).Msg("Error polling")
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

func (p *Poller) poll(ctx context.Context) error {
	log.Debug().Msg("Polling started")
	if err := p.service.ProcessExpiredDelegations(ctx); err != nil {
		log.Error().Err(err).Msg("Error processing expired delegations")
		return err
	}
	log.Debug().Msg("Polling completed")
	return nil
}
