package test

import (
	"testing"

	"github.com/ghettovoice/gossip/transaction"
	"github.com/ghettovoice/gossip/transport"
)

// Tests we can start/stop a transaction manager repeatedly on the same port.
func TestStop(t *testing.T) {
	loops := 5
	for i := 0; i < loops; i++ {
		tp, err := transport.NewManager("udp")
		if err != nil {
			t.Fatalf("Failed to start transport manager on loop %v: %v\n", i, err)
		}

		m, err := transaction.NewManager(tp, "localhost:12345")
		if err != nil {
			t.Fatalf("Failed to start transaction manager on loop %v: %v\n", i, err)
		}

		m.Stop()
	}
}
