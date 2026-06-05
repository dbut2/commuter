package parkrun

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestLookup(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	name, found, err := Lookup(ctx, DefaultBaseURL, 6364752, "25/04/2026")
	if err != nil {
		t.Skipf("parkrun not reachable from this environment: %v", err)
	}

	assert.True(t, found)
	assert.Equal(t, "Toolern Creek parkrun", name)
}
