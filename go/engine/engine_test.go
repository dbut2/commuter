package engine

import (
	"context"
	"testing"
	"time"

	"dbut.dev/commuter/go/core"
	"dbut.dev/commuter/go/provider"
)

func testActivity() core.Activity {
	return core.Activity{
		Name:           "Morning Run",
		Type:           "Run",
		Distance:       12000, // metres
		MovingDuration: 50 * time.Minute,
		Time:           time.Date(2026, 6, 6, 9, 1, 0, 0, time.UTC),
	}
}

func eng() *Engine { return New(provider.Strava{}) }

func TestEvaluate_EnumAndNumberMatch(t *testing.T) {
	r := Rule{Conds: []Cond{
		{Field: "strava.activity_type", Op: "eq", Value: "Run"},
		{Field: "strava.activity_distance", Op: "gt", Value: "10"},
	}}
	got, err := eng().Evaluate(context.Background(), r, testActivity())
	if err != nil {
		t.Fatal(err)
	}
	if !got.Matched {
		t.Fatalf("expected match, got %+v", got)
	}
}

func TestEvaluate_NumberNoMatch(t *testing.T) {
	r := Rule{Conds: []Cond{
		{Field: "strava.activity_distance", Op: "gt", Value: "20"},
	}}
	got, err := eng().Evaluate(context.Background(), r, testActivity())
	if err != nil {
		t.Fatal(err)
	}
	if got.Matched {
		t.Fatalf("expected no match, got %+v", got)
	}
}

func TestEvaluate_EnumNotEqual(t *testing.T) {
	r := Rule{Conds: []Cond{
		{Field: "strava.activity_type", Op: "neq", Value: "Ride"},
	}}
	got, _ := eng().Evaluate(context.Background(), r, testActivity())
	if !got.Matched {
		t.Fatalf("expected match (Run != Ride)")
	}
}

func TestEvaluate_DurationGreater(t *testing.T) {
	r := Rule{Conds: []Cond{
		{Field: "strava.activity_moving_duration", Op: "gt", Value: "45:00"},
	}}
	got, err := eng().Evaluate(context.Background(), r, testActivity())
	if err != nil {
		t.Fatal(err)
	}
	if !got.Matched {
		t.Fatalf("expected match (50m > 45m)")
	}
}

func TestApply_TitleTemplateAndFlags(t *testing.T) {
	a := testActivity()
	r := Rule{Acts: []Act{
		{Key: "title", Template: "Run | {strava.activity_distance}km"},
		{Key: "commute", Template: "true"},
	}}
	res, err := eng().Apply(context.Background(), r, &a)
	if err != nil {
		t.Fatal(err)
	}
	if res.Pending {
		t.Fatalf("did not expect pending")
	}
	if a.Name != "Run | 12km" {
		t.Fatalf("title = %q", a.Name)
	}
	if !a.Commute {
		t.Fatalf("expected commute set")
	}
}

type fakeExternal struct{}

func (fakeExternal) Name() string { return "parkrun" }
func (fakeExternal) Data() []core.Field {
	return []core.Field{{
		Name: "position",
		Type: core.TypeNumber.DataType,
		Fetch: func(context.Context, core.Activity) (any, bool, error) {
			return nil, false, nil // not found yet
		},
	}}
}

func TestApply_PendingWhenExternalMissing(t *testing.T) {
	e := New(provider.Strava{}, fakeExternal{})
	a := testActivity()
	r := Rule{Acts: []Act{{Key: "title", Template: "parkrun #{parkrun.position}"}}}
	res, err := e.Apply(context.Background(), r, &a)
	if err != nil {
		t.Fatal(err)
	}
	if !res.Pending {
		t.Fatalf("expected pending when external field missing")
	}
	if a.Name == "parkrun #" {
		t.Fatalf("activity must be left unchanged while pending")
	}
}

type countingExternal struct{ calls *int }

func (countingExternal) Name() string { return "parkrun" }
func (c countingExternal) Data() []core.Field {
	return []core.Field{{
		Name: "position",
		Type: core.TypeNumber.DataType,
		Fetch: func(context.Context, core.Activity) (any, bool, error) {
			*c.calls++
			return nil, false, nil
		},
	}}
}

func TestEvaluate_ShortCircuitsBeforeNotReadyField(t *testing.T) {
	calls := 0
	e := New(provider.Strava{}, countingExternal{&calls})
	r := Rule{Conds: []Cond{
		{Field: "strava.activity_type", Op: "eq", Value: "Ride"},
		{Field: "parkrun.position", Op: "gt", Value: "0"},
	}}
	got, err := e.Evaluate(context.Background(), r, testActivity())
	if err != nil {
		t.Fatal(err)
	}
	if got.Matched || got.Pending {
		t.Fatalf("expected no match / no pending, got %+v", got)
	}
	if calls != 0 {
		t.Fatalf("the not-ready field was fetched %d times after a definitive fail; want 0", calls)
	}
}

func TestEvaluate_NotReadyFieldIsPending(t *testing.T) {
	calls := 0
	e := New(provider.Strava{}, countingExternal{&calls})
	r := Rule{Conds: []Cond{
		{Field: "strava.activity_type", Op: "eq", Value: "Run"}, // passes
		{Field: "parkrun.position", Op: "gt", Value: "0"},       // not ready
	}}
	got, err := e.Evaluate(context.Background(), r, testActivity())
	if err != nil {
		t.Fatal(err)
	}
	if !got.Pending {
		t.Fatalf("expected pending when a matched rule's field isn't ready, got %+v", got)
	}
}

type foundExternal struct{}

func (foundExternal) Name() string { return "parkrun" }
func (foundExternal) Data() []core.Field {
	return []core.Field{{
		Name: "position",
		Type: core.TypeNumber.DataType,
		Fetch: func(context.Context, core.Activity) (any, bool, error) {
			return "42", true, nil
		},
	}}
}

func TestRun_FetchedRecordsUsedProviders(t *testing.T) {
	e := New(provider.Strava{}, foundExternal{})
	run := e.NewRun()
	r := Rule{
		Conds: []Cond{{Field: "strava.activity_type", Op: "eq", Value: "Run"}},
		Acts:  []Act{{Key: "title", Template: "x {parkrun.position}"}},
	}
	act := testActivity()
	if _, err := run.Evaluate(context.Background(), r, act); err != nil {
		t.Fatal(err)
	}
	if _, err := run.Apply(context.Background(), r, &act); err != nil {
		t.Fatal(err)
	}

	f := run.Fetched()
	if _, ok := f["strava"]; !ok {
		t.Fatalf("expected strava in fetched set: %+v", f)
	}
	pk, ok := f["parkrun"]
	if !ok || !pk.Found || pk.Values["position"] != "42" {
		t.Fatalf("expected parkrun.position cached as 42: %+v", f)
	}
	if _, ok := f["geocode"]; ok {
		t.Fatalf("geocode should not be fetched when unused")
	}
}

func TestEvaluate_PendingExternalCondition(t *testing.T) {
	e := New(provider.Strava{}, fakeExternal{})
	r := Rule{Conds: []Cond{{Field: "parkrun.position", Op: "gt", Value: "0"}}}
	got, err := e.Evaluate(context.Background(), r, testActivity())
	if err != nil {
		t.Fatal(err)
	}
	if !got.Pending {
		t.Fatalf("expected pending, got %+v", got)
	}
}
