package transport

import (
	"net"

	"github.com/ghettovoice/gossip/base"
	"github.com/ghettovoice/gossip/log"
	"github.com/ghettovoice/gossip/parser"
	"github.com/ghettovoice/gossip/utils"
)

type Udp struct {
	listeningPoints []*net.UDPConn
	output          chan base.SipMessage
	stop            bool
}

func NewUdp(output chan base.SipMessage) (*Udp, error) {
	newUdp := Udp{listeningPoints: make([]*net.UDPConn, 0), output: output}
	return &newUdp, nil
}

func (udp *Udp) Listen(address string) error {
	addr, err := net.ResolveUDPAddr("udp", address)
	if err != nil {
		return err
	}

	lp, err := net.ListenUDP("udp", addr)

	if err == nil {
		udp.listeningPoints = append(udp.listeningPoints, lp)
		go udp.listen(lp)
	}

	return err
}

func (udp *Udp) IsStreamed() bool {
	return false
}

func (udp *Udp) Send(addr string, msg base.SipMessage) error {
	msg.Log().Infof("sending message to %v: %v", addr, msg.Short())
	msg.Log().Debugf("sending message:\r\n%v", msg.String())

	raddr, err := net.ResolveUDPAddr("udp", addr)
	if err != nil {
		return err
	}

	var conn *net.UDPConn
	conn, err = net.DialUDP("udp", nil, raddr)
	if err != nil {
		return err
	}
	defer conn.Close()

	_, err = conn.Write([]byte(msg.String()))

	return err
}

func (udp *Udp) listen(conn *net.UDPConn) {
	log.Infof("begin listening for UDP on address %s", conn.LocalAddr())

	buffer := make([]byte, c_BUFSIZE)
	iter := func(conn *net.UDPConn, buffer []byte) bool {
		logger := log.WithField("packet-tag", utils.RandStr(4, "pkt-"))
		// eat bytes
		num, _, err := conn.ReadFromUDP(buffer)
		if err != nil {
			if udp.stop {
				log.Infof("stopped listening for UDP on %s", conn.LocalAddr)
				return false
			} else {
				logger.Errorf("failed to read from UDP buffer: %s", err.Error())
				return true
			}
		}

		pkt := append([]byte(nil), buffer[:num]...)
		go func() {
			msg, err := parser.ParseMessage(pkt, logger)
			if err != nil {
				logger.Warnf("failed to parse SIP message: %s", err.Error())
			} else {
				udp.output <- msg
			}
		}()

		return true
	}
	for {
		if !iter(conn, buffer) {
			break
		}
	}
}

func (udp *Udp) Stop() {
	udp.stop = true
	for _, lp := range udp.listeningPoints {
		lp.Close()
	}
}
