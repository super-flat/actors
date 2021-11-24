package actors

import "time"

// DispatcherOpt helps defines custom options
type DispatcherOpt func(dispatcher *Dispatcher)

// WithPassivationFrequency set the passivation frequency in seconds
func WithPassivationFrequency(passivationFrequency time.Duration) DispatcherOpt {
	return func(dispatcher *Dispatcher) {
		dispatcher.passivationFrequency = passivationFrequency
	}
}

// WithPassivation set how long in seconds an actor should be idle
func WithPassivation(passivateAfterSec time.Duration) DispatcherOpt {
	return func(dispatcher *Dispatcher) {
		dispatcher.maxActorInactivity = passivateAfterSec
	}
}

// WithDispatcherBufferSize sets the dispatcher buffer size
func WithDispatcherBufferSize(bufferSize int) DispatcherOpt {
	return func(dispatcher *Dispatcher) {
		dispatcher.bufferSize = bufferSize
	}
}

// WithAskTimeout set how long in seconds an actor should reply a command
// in an Ask pattern
func WithAskTimeout(askTimeout time.Duration) DispatcherOpt {
	return func(dispatcher *Dispatcher) {
		dispatcher.askTimeout = askTimeout
	}
}
