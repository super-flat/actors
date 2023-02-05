package projection

import (
	"github.com/super-flat/actors/log"
	"github.com/super-flat/actors/persistence"
)

// Config defines a projection config
type Config struct {
	// Name specifies the projection Name
	Name string
	// Logger specifies the logger
	Logger log.Logger
	// Handler specifies the projection handler
	Handler Handler
	// JournalStore specifies the journal store for reading events
	JournalStore persistence.JournalStore
	// OffsetStore specifies the offset store to commit offsets
	OffsetStore persistence.OffsetStore
	// Specifies the recovery setting
	RecoverySetting *RecoverySetting
}
