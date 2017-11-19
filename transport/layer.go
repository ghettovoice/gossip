package transport

import "github.com/ghettovoice/gossip/base"

const (
	bufSize           uint16 = 65535
	listenerQueueSize uint16 = 10000
)

// Layer is an transport layer RFC 3261 - 18.
type Layer interface {
	Listen(addr string) error
	Send(addr string, msg base.SipMessage) error
	Stop()
	// Messages returns channel with incoming messages
	Messages() <-chan base.SipMessage
	// Errors returns transport layers errors
	Errors() <-chan error
}

type layer struct {
	protocols map[Protocol]Protocol
	messages  chan base.SipMessage
	errors    chan error
}

func (tl *layer) Messages() <-chan base.SipMessage {
	return tl.messages
}

func (tl *layer) Errors() <-chan error {
	return tl.errors
}
