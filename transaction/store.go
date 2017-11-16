package transaction

import (
	"fmt"
	"strings"
	"sync"

	"github.com/ghettovoice/gossip/base"
)

type txKey string

// makeServerTxKey creates server transaction key for matching retransmitting requests - RFC 3261 17.2.3.
func makeServerTxKey(req *base.Request) (txKey, error) {
	var sep = "$"

	firstViaHop, err := req.ViaHop()
	if err != nil {
		return "", fmt.Errorf("couldn't create transaction key from request %s: %s", req.Short(), err)
	}

	cseq, err := req.CSeq()
	if err != nil {
		return "", fmt.Errorf("couldn't create transaction key from request %s: %s", req.Short(), err)
	}
	method := cseq.MethodName
	if method == base.ACK {
		method = base.INVITE
	}

	var isRFC3261 bool
	branch, err := req.Branch()
	if err == nil &&
		branch.String() != "" &&
		strings.HasPrefix(branch.String(), base.RFC3261BranchMagicCookie) &&
		strings.TrimPrefix(branch.String(), base.RFC3261BranchMagicCookie) != "" {

		isRFC3261 = true
	} else {
		isRFC3261 = false
	}

	// RFC 3261 compliant
	if isRFC3261 {
		return txKey(strings.Join([]string{
			branch.String(),
			firstViaHop.Host,              // branch
			fmt.Sprint(*firstViaHop.Port), // sent-by
			string(method),                // origin method
		}, sep)), nil
	}
	// RFC 2543 compliant
	fromTag, err := req.FromTag()
	if err != nil {
		return "", fmt.Errorf("couldn't create transaction key from request %s: %s", req.Short(), err)
	}
	callId, err := req.CallId()
	if err != nil {
		return "", fmt.Errorf("couldn't create transaction key from request %s: %s", req.Short(), err)
	}

	return txKey(strings.Join([]string{
		req.Recipient.String(), // request-uri
		fromTag.String(),       // from tag
		callId.String(),        // call-id
		string(method),         // cseq method
		fmt.Sprint(cseq.SeqNo), // cseq num
		firstViaHop.String(),   // top Via
	}, sep)), nil
}

// makeClientTxKey creates client transaction key for matching responses - RFC 3261 17.1.3.
func makeClientTxKey(msg base.SipMessage) (txKey, error) {
	var sep = "$"

	cseq, err := msg.CSeq()
	if err != nil {
		return "", fmt.Errorf("couldn't create transaction key from response %s: %s", msg.Short(), err)
	}
	method := cseq.MethodName
	if method == base.ACK {
		method = base.INVITE
	}

	branch, err := msg.Branch()
	if err != nil {
		return "", fmt.Errorf("couldn't create transaction key from response %s: %s", msg.Short(), err)
	}
	if len(branch.String()) == 0 ||
		!strings.HasPrefix(branch.String(), base.RFC3261BranchMagicCookie) ||
		len(strings.TrimPrefix(branch.String(), base.RFC3261BranchMagicCookie)) == 0 {
		return "", fmt.Errorf("couldn't create transaction key from response %s: empty or malformed 'branch'", msg.Short())
	}

	return txKey(strings.Join([]string{
		branch.String(),
		string(method),
	}, sep)), nil
}

// store is a mutual exclusive storage for active transactions.
type store struct {
	txs    map[txKey]Transaction
	txLock *sync.RWMutex
}

func newStore() *store {
	return &store{
		txs:    make(map[txKey]Transaction),
		txLock: &sync.RWMutex{},
	}
}

func (store *store) putTx(key txKey, tx Transaction) {
	store.txLock.Lock()
	store.txs[key] = tx
	store.txLock.Unlock()
}

// Gets a transaction from the transaction store.
// Should only be called inside the storage handling goroutine to ensure concurrency safety.
func (store *store) getTx(key txKey) (Transaction, bool) {
	store.txLock.RLock()
	tx, ok := store.txs[key]
	store.txLock.RUnlock()

	return tx, ok
}

// Deletes a transaction from the transaction store.
// Should only be called inside the storage handling goroutine to ensure concurrency safety.
func (store *store) delTx(key txKey) {
	store.txLock.Lock()
	delete(store.txs, key)
	store.txLock.Unlock()
}

/* strong typed helpers */

// RFC 17.1.3.
func (store *store) getClientTx(res *base.Response) (*ClientTransaction, error) {
	res.Log().Debugf("trying to get client transaction key from response %s", res.Short())
	key, err := makeClientTxKey(res)
	if err != nil {
		return nil, fmt.Errorf("failed to match response %s to client transaction: %s", res.Short(), err)
	}

	res.Log().Debugf("trying to match response %s to client transaction by key %s", res.Short(), key)
	tx, ok := store.getTx(key)
	if !ok {
		return nil, fmt.Errorf(
			"failed to match response %s to client transaction: transaction with key %s not found",
			res.Short(),
			key,
		)
	}

	switch tx := tx.(type) {
	case *ClientTransaction:
		return tx, nil
	default:
		return nil, fmt.Errorf(
			"failed to match response %s to client transaction: found value at %p is not client transaction",
			res.Short(),
			tx,
		)
	}
}

func (store *store) putClientTx(tx *ClientTransaction) error {
	tx.Log().Debugf("trying to get key of client transaction %p", tx)
	key, err := makeClientTxKey(tx.Origin())
	if err != nil {
		return fmt.Errorf("failed to put client transaction %p: %s", tx, err)
	}

	tx.Log().Debugf("trying to store client transaction %p with key %s", tx, key)
	store.putTx(key, tx)

	return nil
}

func (store *store) delClientTx(tx *ClientTransaction) error {
	tx.Log().Debugf("trying to get key of client transaction %p", tx)
	key, err := makeClientTxKey(tx.Origin())
	if err != nil {
		return fmt.Errorf("failed to delete client transaction %p: %s", tx, err)
	}

	tx.Log().Debugf("trying to delete client transaction %p by key %v", tx, key)
	store.delTx(key)

	return nil
}

// RFC 17.2.3.
func (store *store) getServerTx(req *base.Request) (*ServerTransaction, error) {
	req.Log().Debugf("trying to get server transaction key from request %s", req.Short())
	key, err := makeServerTxKey(req)
	if err != nil {
		return nil, fmt.Errorf("failed to match request %s to server transaction: %s", req.Short(), err)
	}

	req.Log().Debugf("trying to match request %s to server transaction by key %s", req.Short(), key)
	tx, ok := store.getTx(key)
	if !ok {
		return nil, fmt.Errorf(
			"failed to match request %s to server transaction: transaction with key %s not found",
			req.Short(),
			key,
		)
	}

	switch tx := tx.(type) {
	case *ServerTransaction:
		return tx, nil
	default:
		return nil, fmt.Errorf(
			"failed to match request %s to server transaction: found value at %p is not server transaction",
			req.Short(),
			tx,
		)
	}
}

func (store *store) putServerTx(tx *ServerTransaction) error {
	tx.Log().Debugf("trying to get key of server transaction %p", tx)
	key, err := makeServerTxKey(tx.Origin())
	if err != nil {
		return fmt.Errorf("failed to put server transaction %p: %s", tx, err)
	}

	tx.Log().Debugf("trying to store server transaction %p with key %s", tx, key)
	store.putTx(key, tx)

	return nil
}

func (store *store) delServerTx(tx *ServerTransaction) error {
	tx.Log().Debugf("trying to get key of server transaction %p", tx)
	key, err := makeServerTxKey(tx.Origin())
	if err != nil {
		return fmt.Errorf("failed to delete server transaction %p: %s", tx, err)
	}

	tx.Log().Debugf("trying to delete server transaction %p by key %v", tx, key)
	store.delTx(key)

	return nil
}
