package transaction

import (
	"fmt"

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
	// not matched responses
	responses chan *base.Response
}

func NewManager(t transport.Manager, addr string) (*Manager, error) {
	mng := &Manager{
		transport: t,
		store:     newStore(),
	}

	mng.requests = make(chan *ServerTransaction, 5)
	mng.responses = make(chan *base.Response, 5)
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

// Responses returns channel where not matched responses arrived directly from trans[prt layer - RFC 3261 - 17.1.1.2.
func (mng *Manager) Responses() <-chan *base.Response {
	return (<-chan *base.Response)(mng.responses)
}

func (mng *Manager) handle(msg base.SipMessage) {
	msg.Log().Infof("received message: %s", msg.Short())
	msg.Log().Debugf("received message:\r\n%s", msg.String())

	switch m := msg.(type) {
	// acts as UAS, Server Transaction - RFC 3261 17.2
	case *base.Request:
		mng.request(m)
	// acts as UAC, Client Transaction - RFC 3261 17.1
	case *base.Response:
		mng.correlate(m)
	default:
		msg.Log().Warnf("unsupported message type %s", msg.Short())
	}
}

// Create Client transaction.
func (mng *Manager) Send(req *base.Request, dest string) *ClientTransaction {
	req.Log().Infof("sending request to %v: %v", dest, req.Short())
	req.Log().Debugf("sending request:\r\n%s", req.String())

	tx := NewClientTransaction(req, dest, mng.transport)
	tx.Init()

	err := mng.transport.Send(dest, req)
	if err != nil {
		tx.transportError(err, req)
		//err = fmt.Errorf("failed to send request %s: %s", req.Short(), err)
		//tx.Log().Warn(err)
		//tx.lastErr = err
		//tx.fsm.Spin(client_input_transport_err)
	}

	if err := mng.putClientTx(tx); err != nil {
		err = fmt.Errorf("failed to store client transaction %p: %s", tx, err)
		tx.Log().Error(err)
		tx.lastErr = err
		// TODO should tx transition to terminated state?
		//tx.fsm.Spin(client_state_terminated)
	}

	return tx
}

// Give a received response to the correct transaction.
func (mng *Manager) correlate(res *base.Response) {
	tx, err := mng.getClientTx(res)
	if err != nil {
		res.Log().Warn(err)
		// RFC 3261 - 17.1.1.2.
		// Not matched responses should be passed directly to the UA
		mng.responses <- res
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

	// RFC 3261 8.2.6.1
	// UASs SHOULD NOT issue a provisional response for a non-INVITE request.
	// Rather, UASs SHOULD generate a final response to a non-INVITE request as soon as possible.
	if req.Method == base.INVITE {
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
	tx.Trying()
}
