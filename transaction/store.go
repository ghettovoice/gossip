package transaction

import (
	"fmt"
	"strings"

	"github.com/ghettovoice/gossip/base"
)

type txKey []string

type store struct {
	txs map[key]Transaction
}

func makeTxKey(msg base.SipMessage) (txKey, error) {
	firstViaHop, err := msg.ViaHop()
	if err != nil {
		return nil, fmt.Errorf("couldn't create transaction key: %s", err)
	}

	switch msg := msg.(type) {
	case *base.Request:
		method := msg.Method
		if method == base.ACK {
			method = base.INVITE
		}

		if branch, err := msg.Branch(); err == nil {
			if branchStr, ok := branch.(base.String); ok && branchStr.String() != "" &&
				strings.HasPrefix(branchStr.String(), RFC3261MagicCookie) &&
				strings.TrimPrefix(branchStr.String(), RFC3261MagicCookie) != "" {
				// RFC3261 compliant
				return txKey{
					branch,
					firstViaHop,
					method,
				}, nil
			}
		}
		// RFC 2543 compliant
		fromTag, err := msg.FromTag()
		if err != nil {
			return nil, fmt.Errorf("couldn't create transaction id: %s", err)
		}
		toTag, err := msg.ToTag()
		if err != nil {
			return nil, fmt.Errorf("couldn't create transaction id: %s", err)
		}
		callId, err := msg.CallId()
		if err != nil {
			return nil, fmt.Errorf("couldn't create transaction id: %s", err)
		}
		cseq, err := msg.CSeq()
		if err != nil {
			return nil, fmt.Errorf("couldn't create transaction id: %s", err)
		}

		return txKey{
			msg.Recipient.String(),
			toTag,
			fromTag,
			callId,
			firstViaHop,
			method,
			cseq.SeqNo,
		}, nil
	case *base.Response:
		// todo
	default:
		return nil, fmt.Errorf("unsupported message type")
	}
}
