package actors

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/super-flat/actors/log"
	pb "github.com/super-flat/actors/pb/actors/v1"
)

func TestPIDOptions(t *testing.T) {
	testCases := []struct {
		name           string
		option         pidOption
		expectedConfig *pid
	}{
		{
			name:           "WithPassivationAfter",
			option:         withPassivationAfter(time.Second),
			expectedConfig: &pid{passivateAfter: time.Second},
		},
		{
			name:           "WithSendReplyTimeout",
			option:         withSendReplyTimeout(time.Second),
			expectedConfig: &pid{sendReplyTimeout: time.Second},
		},
		{
			name:           "WithInitMaxRetries",
			option:         withInitMaxRetries(5),
			expectedConfig: &pid{initMaxRetries: 5},
		},
		{
			name:           "WithLogger",
			option:         withCustomLogger(log.DefaultLogger),
			expectedConfig: &pid{logger: log.DefaultLogger},
		},
		{
			name:           "WithSupervisorStrategy",
			option:         withSupervisorStrategy(pb.StrategyDirective_RESTART_DIRECTIVE),
			expectedConfig: &pid{supervisorStrategy: pb.StrategyDirective_RESTART_DIRECTIVE},
		},
		{
			name:           "WithShutdownTimeout",
			option:         withShutdownTimeout(time.Second),
			expectedConfig: &pid{shutdownTimeout: time.Second},
		},
		{
			name:           "WithPassivationDisabled",
			option:         withPassivationDisabled(),
			expectedConfig: &pid{passivateAfter: -1},
		},
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			pid := &pid{}
			tc.option(pid)
			assert.Equal(t, tc.expectedConfig, pid)
		})
	}
}
