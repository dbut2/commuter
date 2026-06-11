package engine

import (
	"context"
	"testing"

	"dbut.dev/commuter/go/core"
	"dbut.dev/commuter/go/provider"
)

func varsEng() *Engine {
	return New(provider.Strava{}, provider.Vars{Defs: []provider.VarDef{
		{Name: "home", Kind: core.KindCoords, Value: "-37.6832, 144.5810"},
		{Name: "min_km", Kind: core.KindNumber, Value: "10"},
	}})
}

func nearHomeActivity() core.Activity {
	a := testActivity()
	a.StartLoc = [2]float64{-37.6830, 144.5812}
	a.EndLoc = [2]float64{-37.8136, 144.9631}
	return a
}

func TestEvaluate_IsNearCoordsVariable(t *testing.T) {
	r := Rule{Conds: []Cond{
		{Field: "strava.activity_start_location", Op: "near", Value: "{vars.home} 300m"},
	}}
	got, err := varsEng().Evaluate(context.Background(), r, nearHomeActivity())
	if err != nil {
		t.Fatal(err)
	}
	if !got.Matched {
		t.Fatalf("expected start near home to match, got %+v", got)
	}
}

func TestEvaluate_IsNearNoMatchFarAway(t *testing.T) {
	r := Rule{Conds: []Cond{
		{Field: "strava.activity_end_location", Op: "near", Value: "{vars.home} 300m"},
	}}
	got, err := varsEng().Evaluate(context.Background(), r, nearHomeActivity())
	if err != nil {
		t.Fatal(err)
	}
	if got.Matched {
		t.Fatalf("end is ~35km away, expected no match")
	}
	if got.Failed == nil || got.Failed.Field != "strava.activity_end_location" {
		t.Fatalf("expected failed cond to be reported, got %+v", got.Failed)
	}
}

func TestEvaluate_NumberVariableInCondValue(t *testing.T) {
	r := Rule{Conds: []Cond{
		{Field: "strava.activity_distance", Op: "gte", Value: "{vars.min_km}"},
	}}
	got, err := varsEng().Evaluate(context.Background(), r, nearHomeActivity())
	if err != nil {
		t.Fatal(err)
	}
	if !got.Matched {
		t.Fatalf("distance 12000 >= 10: expected match, got %+v", got)
	}
}

func TestEvaluate_VariableInTemplate(t *testing.T) {
	r := Rule{
		Conds: []Cond{{Field: "strava.activity_type", Op: "eq", Value: "Run"}},
		Acts:  []Act{{Key: "title", Template: "goal {vars.min_km} km"}},
	}
	a := nearHomeActivity()
	res, err := varsEng().Apply(context.Background(), r, &a)
	if err != nil {
		t.Fatal(err)
	}
	if res.Pending {
		t.Fatalf("vars are always available, got pending")
	}
	if a.Name != "goal 10 km" {
		t.Fatalf("title = %q", a.Name)
	}
}

func TestEvaluate_IsOneOf(t *testing.T) {
	r := Rule{Conds: []Cond{
		{Field: "strava.activity_type", Op: "in", Value: "Run, Walk"},
	}}
	got, err := eng().Evaluate(context.Background(), r, testActivity())
	if err != nil {
		t.Fatal(err)
	}
	if !got.Matched {
		t.Fatalf("Run is one of (Run, Walk): expected match, got %+v", got)
	}
}
