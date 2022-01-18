package server

import (
	"context"
	"github.com/ChronosX88/yans/internal/protocol"
	"io"
	"log"
	"net"
	"net/textproto"
)

type SessionMode int

const (
	SessionModeTransit = iota
	SessionModeReader
)

type Session struct {
	ctx          context.Context
	capabilities protocol.Capabilities
	conn         net.Conn
	tconn        *textproto.Conn
	id           string
	closed       chan<- bool
	h            *Handler
	mode         SessionMode
}

func NewSession(ctx context.Context, conn net.Conn, caps protocol.Capabilities, id string, closed chan<- bool, handler *Handler) (*Session, error) {
	var err error
	defer func() {
		if err != nil {
			conn.Close()
			close(closed)
		}
	}()

	tconn := textproto.NewConn(conn)
	s := &Session{
		ctx:          ctx,
		conn:         conn,
		tconn:        tconn,
		capabilities: caps,
		id:           id,
		closed:       closed,
		h:            handler,
		mode:         SessionModeTransit,
	}

	go s.loop()

	return s, nil
}

func (s *Session) loop() {
	defer func() {
		close(s.closed)
	}()

	err := s.tconn.PrintfLine(protocol.MessageNNTPServiceReadyPostingProhibited) // by default access mode is read-only
	if err != nil {
		s.conn.Close()
		return
	}

	for {
		select {
		case <-s.ctx.Done():
			break
		default:
			{
				message, err := s.tconn.ReadLine()
				if err != nil {
					if err == io.EOF || err.(*net.OpError).Unwrap() == net.ErrClosed {
						log.Printf("Client %s has diconnected!", s.conn.RemoteAddr().String())
					} else {
						log.Print(err)
						s.conn.Close()
					}
					return
				}
				log.Printf("Received message from %s: %s", s.conn.RemoteAddr().String(), message) // for debugging
				err = s.h.Handle(s, message)
				if err != nil {
					log.Print(err)
					s.tconn.PrintfLine("%s %s", protocol.MessageErrorHappened, err.Error())
					s.conn.Close()
					return
				}
			}
		}
	}

}
