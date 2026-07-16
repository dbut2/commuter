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

	segments, err := p.repo.Segments(ctx, userID)
	if err != nil {
		return nil, err
	}
	if len(segments) > 0 {
		defs := make([]provider.SegmentDef, len(segments))
		for i, s := range segments {
			defs[i] = provider.SegmentDef{Name: s.Name, SegmentID: s.SegmentID}
		}
		providers = append(providers, provider.Segments{Defs: defs})
	}

	vars, err := p.repo.Vars(ctx, userID)
	if err != nil {
		return nil, err
	}
	defs := make([]provider.VarDef, len(vars))
	for i, v := range vars {
		defs[i] = provider.VarDef{Name: v.Name, Kind: core.Kind(v.Type), Value: v.Value}
	}
	providers = append(providers, provider.Vars{Defs: defs})

	return providers, nil
}

func (p *Processor) Catalog(ctx context.Context, userID string) ([]core.Provider, error) {
	return p.userProviders(ctx, userID)
}

func missingProviders(needed []string, available map[string]bool) []string {
	var missing []string
	for _, p := range needed {
		if !available[p] {
			missing = append(missing, p)
		}
	}
	return missing
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
