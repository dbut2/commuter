package core

import "testing"

func TestParseNearTarget(t *testing.T) {
	cases := []struct {
		in     string
		coords [2]float64
		radius float64
	}{
		{"-37.68, 144.58", [2]float64{-37.68, 144.58}, 300},
		{"-37.68, 144.58 500m", [2]float64{-37.68, 144.58}, 500},
		{"-37.68,144.58 1.5km", [2]float64{-37.68, 144.58}, 1500},
		{"-37.68, 144.58 250", [2]float64{-37.68, 144.58}, 250},
	}
	for _, c := range cases {
		got, err := ParseNearTarget(c.in)
		if err != nil {
			t.Fatalf("%q: %v", c.in, err)
		}
		if got.Coords != c.coords || got.Radius != c.radius {
			t.Fatalf("%q: got %+v", c.in, got)
		}
	}
	if _, err := ParseNearTarget("not coords"); err == nil {
		t.Fatal("expected error for junk input")
	}
}

func TestHaversineMetres(t *testing.T) {
	mel := [2]float64{-37.8136, 144.9631}
	syd := [2]float64{-33.8688, 151.2093}
	d := HaversineMetres(mel, syd)
	if d < 700_000 || d > 730_000 {
		t.Fatalf("Melbourne–Sydney = %.0f m, want ~714 km", d)
	}
	if HaversineMetres(mel, mel) != 0 {
		t.Fatal("identical points should be 0 m apart")
	}
}

func TestParseValueKinds(t *testing.T) {
	if _, err := ParseValue(KindCoords, "-37.68, 144.58"); err != nil {
		t.Fatal(err)
	}
	if _, err := ParseValue(KindNumber, "300"); err != nil {
		t.Fatal(err)
	}
	if _, err := ParseValue(KindNumber, "abc"); err == nil {
		t.Fatal("expected error for non-number")
	}
	if _, err := ParseValue(KindBool, "true"); err != nil {
		t.Fatal(err)
	}
	if _, err := ParseValue(KindTime, "7:45 am"); err != nil {
		t.Fatal(err)
	}
}
