package factorydaemon

import (
	"context"
	"fmt"
	"time"
)

func (s Service) renewLease(ctx context.Context, opts StartOptions, errCh chan<- error) {
	ticker := time.NewTicker(opts.RenewInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			now := s.now()
			_, ok, err := s.Backend.Factory.RenewDaemonLease(
				ctx,
				opts.DaemonID,
				now,
				now.Add(opts.LeaseTTL),
			)
			if err != nil {
				sendRenewErr(ctx, errCh, err)
				return
			}
			if !ok {
				sendRenewErr(
					ctx,
					errCh,
					fmt.Errorf("daemon lease %q is no longer active", opts.DaemonID),
				)
				return
			}
		}
	}
}

func sendRenewErr(ctx context.Context, errCh chan<- error, err error) {
	select {
	case errCh <- err:
	case <-ctx.Done():
	}
}
