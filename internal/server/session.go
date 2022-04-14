package server

import (
	"context"
	"errors"
	"fmt"
	"github.com/ChronosX88/yans/internal/models"
	"github.com/ChronosX88/yans/internal/protocol"
	"io"
	"log"
	"net"
	"net/textproto"
	"strings"
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
	remoteAddr   string
	id           string
	closed       chan<- bool
	h            *Handler

	currentGroup   *models.Group
	currentArticle *models.Article
	mode           SessionMode
}

func NewSession(
	ctx context.Context,
	conn net.Conn,
	remoteAddr string,
	caps protocol.Capabilities,
	id string,
	closed chan<- bool,
	handler *Handler,
) (*Session, error) {
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
		remoteAddr:   remoteAddr,
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

	err := s.tconn.PrintfLine(protocol.NNTPResponse{Code: 201, Message: "YANS NNTP Service Ready, posting allowed"}.String()) // by default access mode is read-only
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
				id := s.tconn.Next()
				s.tconn.StartRequest(id)
				message, err := s.tconn.ReadLine()
				if err != nil {
					if err == io.EOF || errors.Is(err, net.ErrClosed) || strings.Contains(err.Error(), "StatusNormalClosure") {
						log.Printf("Client %s has diconnected!", s.remoteAddr)
					} else {
						log.Print(err)
						s.conn.Close()
					}
					return
				}
				s.tconn.EndRequest(id)
				log.Printf("Received message from %s: %s", s.remoteAddr, message) // for debugging
				err = s.h.Handle(s, message, id)
				if err != nil {
					log.Print(err)
					s.tconn.PrintfLine(protocol.NNTPResponse{Code: 403, Message: fmt.Sprintf("Failed to process command: %s", err.Error())}.String())
					s.conn.Close()
					return
				}
			}
		}
	}

}
