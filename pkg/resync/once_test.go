package resync

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestOnceReset(t *testing.T) {
	// set the number of calls to track
	var calls int
	var c Once
	c.Do(func() {
		calls++
	})
	c.Do(func() {
		calls++
	})
	c.Do(func() {
		calls++
	})
	assert.Equal(t, calls, 1)
	c.Reset()
	c.Do(func() {
		calls++
	})
	c.Do(func() {
		calls++
	})
	c.Do(func() {
		calls++
	})
	assert.Equal(t, calls, 2)
}
