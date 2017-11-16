package transaction

import (
	"fmt"
	"time"

	"github.com/ghettovoice/gossip/base"
	"github.com/ghettovoice/gossip/log"
	"github.com/ghettovoice/gossip/timing"
	"github.com/ghettovoice/gossip/transport"
)

var (
	global *Manager = &Manager{
		store: newStore(),
	}
)

type Manager struct {
	*store
	transport transport.Manager
	requests  chan *ServerTransaction
}

func NewManager(t transport.Manager, addr string) (*Manager, error) {
	mng := &Manager{
		transport: t,
		store:     newStore(),
	}

	mng.requests = make(chan *ServerTransaction, 5)
	log.Debug("run transaction manager")
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
	log.Debug("stop transaction manager")
	// Stop the transport layer.
	mng.transport.Stop()
}

func (mng *Manager) Requests() <-chan *ServerTransaction {
	return (<-chan *ServerTransaction)(mng.requests)
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
		msg.Log().Warnf("unsupported message type %s", msg.Short())
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

	if err := mng.putClientTx(tx); err != nil {
		tx.Log().Warn(err)
	}

	return tx
}

// Give a received response to the correct transaction.
func (mng *Manager) correlate(res *base.Response) {
	tx, err := mng.getClientTx(res)
	if err != nil {
		res.Log().Warn(err)
		return
	}

	tx.Log().Debugf("found client transaction %p, receive response %s", tx, res.Short())
	tx.Receive(res)
}

// Handle a request.
func (mng *Manager) request(req *base.Request) {
	tx, err := mng.getServerTx(req)
	if err == nil {
		tx.Log().Debugf("found server transaction %p, receive request %s", tx, req.Short())
		tx.Receive(req)
		return
	}

	req.Log().Debugf("creating new server transaction for request %s", req.Short())
	// Create a new transaction
	tx = &ServerTransaction{}
	tx.tm = mng
	tx.origin = req
	tx.transport = mng.transport

	// Use the remote address in the top Via header.  This is not correct behaviour.
	port := uint16(5060)
	hop, err := req.ViaHop()
	if err != nil {
		tx.Log().Warnf("failed to process request %s: %s transaction will be dropped", req.Short(), err)
		return
	}

	if hop.Port != nil {
		port = *hop.Port
	}

	tx.dest = fmt.Sprintf("%s:%d", hop.Host, port)
	tx.transport = mng.transport

	tx.initFSM()

	tx.tu = make(chan *base.Response, 3)
	tx.tu_err = make(chan error, 1)
	tx.ack = make(chan *base.Request, 1)

	if req.Method != base.ACK {
		// Send a 100 Trying immediately.
		// Technically we shouldn't do this if we trust the user to do it within 200ms,
		// but I'm not sure how to handle that situation right now.
		// Explicitly don't do this for ACKs; 2xx ACKs are their own transaction but
		// don't engender a provisional response - we just pass them up to the user
		// to handle at the dialog scope.
		mng.sendPresumptiveTrying(tx)
	}

	// put tx to store, to match retransmitting requests later
	// todo check RFC for ACK
	mng.putServerTx(tx)

	mng.requests <- tx
}

func (mng *Manager) sendPresumptiveTrying(tx *ServerTransaction) {
	tx.Log().Infof("sending '100 Trying' auto response on transaction %p", tx)
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
