package processor

import (
	"context"
	"strconv"

	"dbut.dev/commuter/go/core"
	"dbut.dev/commuter/go/provider"
)

func (p *Processor) userProviders(ctx context.Context, userID string) ([]core.Provider, error) {
	providers := []core.Provider{
		provider.Strava{},
		provider.Geocode{},
	}

	parkrunID, err := p.repo.ParkrunID(ctx, userID)
	if err != nil {
		return nil, err
	}
	if parkrunID != "" {
		providers = append(providers, provider.Parkrun{ParkrunID: parseParkrunID(parkrunID)})
	}

	return providers, nil
}

func providersAvailable(needed []string, available map[string]bool) bool {
	for _, p := range needed {
		if !available[p] {
			return false
		}
	}
	return true
}

func parseParkrunID(s string) int64 {
	digits := make([]rune, 0, len(s))
	for _, r := range s {
		if r >= '0' && r <= '9' {
			digits = append(digits, r)
		}
	}
	n, _ := strconv.ParseInt(string(digits), 10, 64)
	return n
}
