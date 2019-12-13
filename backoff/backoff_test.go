package backoff

import (
	"math/rand"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestBackoff(t *testing.T) {
	t.Run("jittered", func(t *testing.T) {
		random := rand.New(rand.NewSource(1))
		b := New(
			WithInitialDelay(time.Second),
			WithMaxDelay(time.Minute),
			WithMultiplier(2),
			WithRandom(random),
			WithJitter(true),
		)
		assert.Equal(t, time.Second, b.Next())
		assert.Equal(t, time.Duration(1940509088), b.Next())
		assert.Equal(t, time.Duration(2993680159), b.Next())
		assert.Equal(t, time.Duration(4063999310), b.Next())
		assert.Equal(t, time.Duration(7369562456), b.Next())
		assert.Equal(t, time.Duration(22291515258), b.Next())
		assert.Equal(t, time.Duration(4872584133), b.Next())
		assert.Equal(t, time.Duration(10234636029), b.Next())
		assert.Equal(t, time.Duration(6721201615), b.Next())
		assert.Equal(t, time.Minute, b.cur)
	})
	t.Run("non-jittered", func(t *testing.T) {
		random := rand.New(rand.NewSource(1))
		b := New(
			WithInitialDelay(time.Second),
			WithMaxDelay(time.Minute),
			WithMultiplier(2),
			WithRandom(random),
			WithJitter(false),
		)
		assert.Equal(t, time.Second, b.Next())
		assert.Equal(t, time.Duration(2000000000), b.Next())
		assert.Equal(t, time.Duration(4000000000), b.Next())
		assert.Equal(t, time.Duration(8000000000), b.Next())
		assert.Equal(t, time.Duration(16000000000), b.Next())
		assert.Equal(t, time.Duration(32000000000), b.Next())
		assert.Equal(t, time.Duration(60000000000), b.Next())
	})
}
