package config

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/tochemey/goakt/log"
)

func TestOptions(t *testing.T) {
	testCases := []struct {
		name           string
		option         Option
		expectedConfig Config
	}{
		{
			name:           "WithExpireActorAfter",
			option:         WithExpireActorAfter(2 * time.Second),
			expectedConfig: Config{expireActorAfter: 2. * time.Second},
		},
		{
			name:           "WithReplyTimeout",
			option:         WithReplyTimeout(2 * time.Second),
			expectedConfig: Config{replyTimeout: 2. * time.Second},
		},
		{
			name:           "WithActorInitMaxRetries",
			option:         WithActorInitMaxRetries(2),
			expectedConfig: Config{actorInitMaxRetries: 2},
		},
		{
			name:           "WithLogger",
			option:         WithLogger(log.DefaultLogger),
			expectedConfig: Config{logger: log.DefaultLogger},
		},
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			var cfg Config
			tc.option.Apply(&cfg)
			assert.Equal(t, tc.expectedConfig, cfg)
		})
	}
}