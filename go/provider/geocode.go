package provider

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"

	"dbut.dev/commuter/go/core"
)

var _ core.Provider = Geocode{}

type Geocode struct{}

func (g Geocode) Name() string { return "geocode" }

func (g Geocode) Data() []core.Field {
	return []core.Field{
		geocodeField("start_country", "Australia", startLoc, func(p geoPlace) string { return p.country }),
		geocodeField("start_city", "Melbourne", startLoc, func(p geoPlace) string { return p.city }),
		geocodeField("end_country", "Australia", endLoc, func(p geoPlace) string { return p.country }),
		geocodeField("end_city", "Melbourne", endLoc, func(p geoPlace) string { return p.city }),
	}
}

func startLoc(a core.Activity) [2]float64 { return a.StartLoc }
func endLoc(a core.Activity) [2]float64   { return a.EndLoc }

func geocodeField(name, example string, loc func(core.Activity) [2]float64, get func(geoPlace) string) core.Field {
	return core.Field{
		Name:    name,
		Example: example,
		Type:    core.TypeString.DataType,
		Fetch: func(ctx context.Context, a core.Activity) (any, bool, error) {
			ll := loc(a)
			if ll[0] == 0 && ll[1] == 0 {
				return nil, false, nil // no coordinates: data unavailable
			}
			p, err := geocodeLookup(ctx, ll[0], ll[1])
			if err != nil {
				return nil, false, err
			}
			v := get(p)
			if v == "" {
				return nil, false, nil
			}
			return v, true, nil
		},
	}
}

const (
	geocodeEndpoint  = "https://nominatim.openstreetmap.org/reverse"
	geocodeUserAgent = "commuter.dbut.dev (https://github.com/dbut2/commuter)"
	geocodeZoomMin   = 5
	geocodeZoomMax   = 12
)

type geoPlace struct {
	country     string
	countryCode string
	city        string
}

var (
	geoHTTP = &http.Client{Timeout: 15 * time.Second}

	geoRateMu sync.Mutex
	geoNext   time.Time

	geoCacheMu sync.RWMutex
	geoCache   = map[string]geoPlace{}
)

func geocodeLookup(ctx context.Context, lat, lon float64) (geoPlace, error) {
	key := geoGridKey(lat, lon)

	geoCacheMu.RLock()
	p, ok := geoCache[key]
	geoCacheMu.RUnlock()
	if ok {
		return p, nil
	}

	p, err := geocodeResolve(ctx, lat, lon)
	if err != nil {
		return geoPlace{}, err
	}

	geoCacheMu.Lock()
	geoCache[key] = p
	geoCacheMu.Unlock()
	return p, nil
}

func geocodeResolve(ctx context.Context, lat, lon float64) (geoPlace, error) {
	for z := geocodeZoomMin; z <= geocodeZoomMax; z++ {
		raw, err := geocodeFetch(ctx, lat, lon, z)
		if err != nil {
			return geoPlace{}, err
		}
		var n nominatimResponse
		if err := json.Unmarshal(raw, &n); err != nil {
			continue
		}
		if isCityTier(n.AddressType, n.Name) {
			return buildGeoPlace(n), nil
		}
	}
	return geoPlace{}, nil
}

func isCityTier(addresstype, name string) bool {
	switch addresstype {
	case "city", "town", "village":
		return true
	case "county":
		return !strings.HasSuffix(name, " County")
	case "province":
		return !strings.HasSuffix(name, " Prefecture")
	}
	return false
}

type nominatimResponse struct {
	Name        string `json:"name"`
	AddressType string `json:"addresstype"`
	Address     struct {
		Country     string `json:"country"`
		CountryCode string `json:"country_code"`
		ISOLvl3     string `json:"ISO3166-2-lvl3"`
	} `json:"address"`
}

func buildGeoPlace(n nominatimResponse) geoPlace {
	country, code := n.Address.Country, n.Address.CountryCode
	switch n.Address.ISOLvl3 {
	case "CN-HK":
		country, code = "Hong Kong", "hk"
	case "CN-MO":
		country, code = "Macau", "mo"
	}
	return geoPlace{country: country, countryCode: code, city: n.Name}
}

func geocodeFetch(ctx context.Context, lat, lon float64, zoom int) (json.RawMessage, error) {
	if err := geocodeRate(ctx); err != nil {
		return nil, err
	}
	q := url.Values{}
	q.Set("lat", strconv.FormatFloat(lat, 'f', 6, 64))
	q.Set("lon", strconv.FormatFloat(lon, 'f', 6, 64))
	q.Set("format", "jsonv2")
	q.Set("zoom", strconv.Itoa(zoom))
	q.Set("addressdetails", "1")
	q.Set("accept-language", "en")

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, geocodeEndpoint+"?"+q.Encode(), nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", geocodeUserAgent)

	resp, err := geoHTTP.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("nominatim %d: %s", resp.StatusCode, body)
	}
	return body, nil
}

// geocodeRate enforces ~1 request/second (Nominatim's usage policy).
func geocodeRate(ctx context.Context) error {
	geoRateMu.Lock()
	wait := time.Until(geoNext)
	if wait < 0 {
		wait = 0
	}
	geoNext = time.Now().Add(wait + 1100*time.Millisecond)
	geoRateMu.Unlock()

	if wait <= 0 {
		return nil
	}
	select {
	case <-time.After(wait):
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func geoGridKey(lat, lon float64) string {
	return geoCoord(lat) + "," + geoCoord(lon)
}

func geoCoord(v float64) string {
	return strconv.FormatFloat(geoRound(v, 3), 'f', 3, 64)
}

func geoRound(v float64, decimals int) float64 {
	pow := 1.0
	for i := 0; i < decimals; i++ {
		pow *= 10
	}
	if v >= 0 {
		return float64(int64(v*pow+0.5)) / pow
	}
	return float64(int64(v*pow-0.5)) / pow
}
