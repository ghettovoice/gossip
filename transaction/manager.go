package transaction

import (
	"fmt"
	"sync"
	"time"

	"github.com/ghettovoice/gossip/base"
	"github.com/ghettovoice/gossip/log"
	"github.com/ghettovoice/gossip/timing"
	"github.com/ghettovoice/gossip/transport"
)

var (
	global *Manager = &Manager{
		txs: map[key]Transaction{},
	}
)

type Manager struct {
	txs       map[key]Transaction
	transport transport.Manager
	requests  chan *ServerTransaction
	txLock    *sync.RWMutex
}

// Transactions are identified by the branch parameter in the top Via header, and the method. (RFC 3261 17.1.3)
type key struct {
	branch string
	method string
}

func NewManager(t transport.Manager, addr string) (*Manager, error) {
	mng := &Manager{
		txs:       map[key]Transaction{},
		txLock:    &sync.RWMutex{},
		transport: t,
	}

	mng.requests = make(chan *ServerTransaction, 5)

	// Spin up a goroutine to pull messages up from the depths.
	c := mng.transport.GetChannel()
	go func() {
		for msg := range c {
			go mng.handle(msg)
		}
	}()

	err := mng.transport.Listen(addr)
	if err != nil {
		return nil, err
	}

	return mng, nil
}

// Stop the manager and close down all processing on it, losing all transactions in progress.
func (mng *Manager) Stop() {
	// Stop the transport layer.
	mng.transport.Stop()
}

func (mng *Manager) Requests() <-chan *ServerTransaction {
	return (<-chan *ServerTransaction)(mng.requests)
}

func (mng *Manager) putTx(tx Transaction) {
	viaHeaders := tx.Origin().Headers("Via")
	if len(viaHeaders) == 0 {
		tx.Log().Warnf("no Via header on new transaction: transaction will be dropped")
		return
	}

	via, ok := viaHeaders[0].(*base.ViaHeader)
	if !ok {
		// TODO: Handle this better.
		tx.Log().Panic("Headers('Via') returned non-Via header")
	}

	branch, ok := (*via)[0].Params.Get("branch")
	if !ok {
		tx.Log().Warnf("no branch parameter on top Via header: transaction will be dropped")
		return
	}

	var k key
	switch branch := branch.(type) {
	case base.String:
		k = key{branch.String(), string(tx.Origin().Method)}
	case base.NoString:
		tx.Log().Warn("empty branch parameter on top Via header: transaction will be dropped")
		return
	default:
		tx.Log().Warnf("unexpected type of branch value on top Via header: %T", branch)
		return
	}
	mng.txLock.Lock()
	mng.txs[k] = tx
	mng.txLock.Unlock()
}

func (mng *Manager) makeKey(s base.SipMessage) (key, bool) {
	viaHeaders := s.Headers("Via")
	via, ok := viaHeaders[0].(*base.ViaHeader)
	if !ok {
		s.Log().Panic("Headers('Via') returned non-Via header")
	}

	b, ok := (*via)[0].Params.Get("branch")
	if !ok {
		return key{}, false
	}

	branch, ok := b.(base.String)
	if !ok {
		return key{}, false
	}

	var method string
	switch s := s.(type) {
	case *base.Request:
		// Correlate an ACK request to the related INVITE.
		if s.Method == base.ACK {
			method = string(base.INVITE)
		} else {
			method = string(s.Method)
		}
	case *base.Response:
		cseqs := s.Headers("CSeq")
		if len(cseqs) == 0 {
			// TODO - Handle non-existent CSeq
			s.Log().Panic("no CSeq on response!")
		}

		cseq, _ := s.Headers("CSeq")[0].(*base.CSeq)
		method = string(cseq.MethodName)
	}

	return key{branch.String(), method}, true
}

// Gets a transaction from the transaction store.
// Should only be called inside the storage handling goroutine to ensure concurrency safety.
func (mng *Manager) getTx(s base.SipMessage) (Transaction, bool) {
	key, ok := mng.makeKey(s)
	if !ok {
		// TODO: Here we should initiate more intense searching as specified in RFC3261 section 17
		s.Log().Warn("could not correlate message to transaction by branch/method: dropping")
		return nil, false
	}
	s.Log().Debugf("trying to match message to transaction by key %v", key)
	mng.txLock.RLock()
	tx, ok := mng.txs[key]
	mng.txLock.RUnlock()

	return tx, ok
}

// Deletes a transaction from the transaction store.
// Should only be called inside the storage handling goroutine to ensure concurrency safety.
func (mng *Manager) delTx(t Transaction) {
	key, ok := mng.makeKey(t.Origin())
	if !ok {
		t.Log().Debug("could not build lookup key for transaction: is it missing a branch parameter?")
	}

	mng.txLock.Lock()
	delete(mng.txs, key)
	mng.txLock.Unlock()
}

func (mng *Manager) handle(msg base.SipMessage) {
	msg.Log().Infof("received message: %s", msg.Short())
	msg.Log().Debugf("received message:\r\n%s", msg.String())

	switch m := msg.(type) {
	// acts as UAS, Server Transaction - RFC 17.2
	case *base.Request:
		mng.request(m)
	// acts as UAC, Client Transaction - RFC 17.1
	case *base.Response:
		mng.correlate(m)
	default:
		// TODO: Error
	}
}

// Create Client transaction.
func (mng *Manager) Send(r *base.Request, dest string) *ClientTransaction {
	r.Log().Infof("sending message to %v: %v", dest, r.Short())
	r.Log().Debugf("sending message:\r\n%s", r.String())

	tx := &ClientTransaction{}
	tx.origin = r
	tx.dest = dest
	tx.transport = mng.transport
	tx.tm = mng

	tx.initFSM()

	tx.tu = make(chan *base.Response, 3)
	tx.tu_err = make(chan error, 1)

	tx.timer_a_time = T1
	tx.timer_a = timing.AfterFunc(tx.timer_a_time, func() {
		tx.fsm.Spin(client_input_timer_a)
	})
	tx.Log().Debugf("client transaction %p, timer_b set to %v", tx, 64*T1)
	tx.timer_b = timing.AfterFunc(64*T1, func() {
		tx.Log().Debugf("client transaction %p, timer_b fired", tx)
		tx.fsm.Spin(client_input_timer_b)
	})

	// Timer D is set to 32 seconds for unreliable transports, and 0 seconds otherwise.
	tx.timer_d_time = 32 * time.Second

	err := mng.transport.Send(dest, r)
	if err != nil {
		tx.Log().Warnf("failed to send message: %s", err.Error())
		tx.fsm.Spin(client_input_transport_err)
	}

	mng.putTx(tx)

	return tx
}

// Give a received response to the correct transaction.
func (mng *Manager) correlate(r *base.Response) {
	tx, ok := mng.getTx(r)
	if !ok {
		// TODO: Something
		r.Log().Warn("failed to correlate response to active transaction: dropping it.")
		return
	}

	tx.Receive(r)
}

// Handle a request.
func (mng *Manager) request(r *base.Request) {
	t, ok := mng.getTx(r)
	if ok {
		t.Receive(r)
		return
	}

	// Create a new transaction
	tx := &ServerTransaction{}
	tx.tm = mng
	tx.origin = r
	tx.transport = mng.transport

	// Use the remote address in the top Via header.  This is not correct behaviour.
	viaHeaders := tx.Origin().Headers("Via")
	if len(viaHeaders) == 0 {
		tx.Log().Warn("no Via header on new transaction: transaction will be dropped.")
		return
	}

	via, ok := viaHeaders[0].(*base.ViaHeader)
	if !ok {
		tx.Log().Panic("Headers('Via') returned non-Via header!")
	}

	if len(*via) == 0 {
		tx.Log().Warn("via header contained no hops: transaction will be dropped.")
		return
	}

	hop := (*via)[0]

	port := uint16(5060)

	if hop.Port != nil {
		port = *hop.Port
	}

	tx.dest = fmt.Sprintf("%s:%d", hop.Host, port)
	tx.transport = mng.transport

	tx.initFSM()

	tx.tu = make(chan *base.Response, 3)
	tx.tu_err = make(chan error, 1)
	tx.ack = make(chan *base.Request, 1)

	if r.Method != base.ACK {
		// Send a 100 Trying immediately.
		// Technically we shouldn't do this if we trust the user to do it within 200ms,
		// but I'm not sure how to handle that situation right now.
		// Explicitly don't do this for ACKs; 2xx ACKs are their own transaction but
		// don't engender a provisional response - we just pass them up to the user
		// to handle at the dialog scope.
		mng.sendPresumptiveTrying(tx)
	}

	// put tx to store, to match retransmitting requests later
	mng.putTx(tx)

	mng.requests <- tx
}

func (mng *Manager) sendPresumptiveTrying(tx *ServerTransaction) {

	// Pretend the user sent us a 100 to send.
	trying := base.NewResponse(
		"SIP/2.0",
		100,
		"Trying",
		[]base.SipHeader{},
		"",
		log.StandardLogger(),
	)

	base.CopyHeaders("Via", tx.origin, trying)
	base.CopyHeaders("From", tx.origin, trying)
	base.CopyHeaders("To", tx.origin, trying)
	base.CopyHeaders("Call-Id", tx.origin, trying)
	base.CopyHeaders("CSeq", tx.origin, trying)

	tx.lastResp = trying
	tx.fsm.Spin(server_input_user_1xx)
}
