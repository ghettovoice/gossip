package transaction

import (
	"fmt"
	"time"

	"github.com/discoviking/fsm"
	"github.com/ghettovoice/gossip/base"
	"github.com/ghettovoice/gossip/log"
	"github.com/ghettovoice/gossip/transport"
)

const (
	T1 = 500 * time.Millisecond
	T2 = 4 * time.Second
)

type Transaction interface {
	log.WithLocalLogger
	Receive(m base.SipMessage)
	Origin() *base.Request
	Destination() string
	Transport() transport.Manager
	Delete()
}

type transaction struct {
	fsm       *fsm.FSM       // FSM which governs the behavior of this transaction.
	origin    *base.Request  // Request that started this transaction.
	lastResp  *base.Response // Most recently received message.
	dest      string         // Of the form hostname:port
	transport transport.Manager
	tm        *Manager
}

func (tx *transaction) Log() log.Logger {
	return tx.origin.Log().WithField("tx-ptr", fmt.Sprintf("%p", tx))
}

func (tx *transaction) Origin() *base.Request {
	return tx.origin
}

func (tx *transaction) Destination() string {
	return tx.dest
}

func (tx *transaction) Transport() transport.Manager {
	return tx.transport
}
