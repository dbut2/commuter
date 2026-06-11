package core

import (
	"fmt"
	"math"
	"math/big"
	"strconv"
	"strings"
	"time"
)

func ParseValue(kind Kind, s string) (any, error) {
	s = strings.TrimSpace(s)
	switch kind {
	case KindString, KindEnum:
		return s, nil
	case KindNumber:
		r, ok := new(big.Rat).SetString(s)
		if !ok {
			return nil, fmt.Errorf("core: %q is not a number", s)
		}
		return r, nil
	case KindBool:
		return strconv.ParseBool(s)
	case KindDuration:
		return ParseDuration(s)
	case KindDate, KindTime, KindDatetime:
		return ParseTime(s)
	case KindCoords:
		return ParseCoords(s)
	default:
		return s, nil
	}
}

func ParseDuration(s string) (time.Duration, error) {
	if d, err := time.ParseDuration(s); err == nil {
		return d, nil
	}
	parts := strings.Split(s, ":")
	var secs int
	for _, p := range parts {
		n, err := strconv.Atoi(strings.TrimSpace(p))
		if err != nil {
			return 0, fmt.Errorf("core: bad duration %q", s)
		}
		secs = secs*60 + n
	}
	return time.Duration(secs) * time.Second, nil
}

var timeLayouts = []string{"15:04", "3:04pm", "3:04PM", "3:04 pm", "3:04 PM", "02/01/2006", "2006-01-02", time.RFC3339}

func ParseTime(s string) (time.Time, error) {
	for _, l := range timeLayouts {
		if t, err := time.Parse(l, strings.TrimSpace(s)); err == nil {
			return t, nil
		}
	}
	return time.Time{}, fmt.Errorf("core: bad time %q", s)
}

func ParseCoords(s string) ([2]float64, error) {
	parts := strings.SplitN(strings.Trim(strings.TrimSpace(s), "[]()"), ",", 2)
	if len(parts) != 2 {
		return [2]float64{}, fmt.Errorf("core: bad coords %q, want \"lat, lon\"", s)
	}
	lat, err1 := strconv.ParseFloat(strings.TrimSpace(parts[0]), 64)
	lon, err2 := strconv.ParseFloat(strings.TrimSpace(parts[1]), 64)
	if err1 != nil || err2 != nil {
		return [2]float64{}, fmt.Errorf("core: bad coords %q, want \"lat, lon\"", s)
	}
	return [2]float64{lat, lon}, nil
}

type NearTarget struct {
	Coords [2]float64
	Radius float64
}

const defaultNearRadius = 300

func ParseNearTarget(s string) (NearTarget, error) {
	s = strings.TrimSpace(s)
	if i := strings.LastIndexByte(s, ' '); i >= 0 {
		if r, ok := parseRadius(s[i+1:]); ok {
			if coords, err := ParseCoords(s[:i]); err == nil {
				return NearTarget{Coords: coords, Radius: r}, nil
			}
		}
	}
	coords, err := ParseCoords(s)
	if err != nil {
		return NearTarget{}, err
	}
	return NearTarget{Coords: coords, Radius: defaultNearRadius}, nil
}

func parseRadius(s string) (float64, bool) {
	mult := 1.0
	switch {
	case strings.HasSuffix(s, "km"):
		mult, s = 1000, strings.TrimSuffix(s, "km")
	case strings.HasSuffix(s, "m"):
		s = strings.TrimSuffix(s, "m")
	}
	n, err := strconv.ParseFloat(s, 64)
	if err != nil || n <= 0 {
		return 0, false
	}
	return n * mult, true
}

func HaversineMetres(a, b [2]float64) float64 {
	const earthRadius = 6371000.0
	lat1, lon1 := a[0]*math.Pi/180, a[1]*math.Pi/180
	lat2, lon2 := b[0]*math.Pi/180, b[1]*math.Pi/180
	dLat, dLon := lat2-lat1, lon2-lon1
	h := math.Sin(dLat/2)*math.Sin(dLat/2) + math.Cos(lat1)*math.Cos(lat2)*math.Sin(dLon/2)*math.Sin(dLon/2)
	return 2 * earthRadius * math.Asin(math.Sqrt(h))
}
