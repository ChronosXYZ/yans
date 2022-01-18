package server

import (
	"context"
	"fmt"
	"github.com/ChronosX88/yans/internal"
	"github.com/ChronosX88/yans/internal/common"
	"github.com/ChronosX88/yans/internal/config"
	"github.com/ChronosX88/yans/internal/protocol"
	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
	_ "github.com/mattn/go-sqlite3"
	"github.com/pressly/goose/v3"
	"log"
	"net"
	"sync"
)

var (
	Capabilities = protocol.Capabilities{
		{Type: protocol.VersionCapability, Params: "2"},
		{Type: protocol.ImplementationCapability, Params: fmt.Sprintf("%s %s", common.ServerName, common.ServerVersion)},
		{Type: protocol.ModeReaderCapability},
		{Type: protocol.ListCapability, Params: "ACTIVE NEWSGROUPS"},
	}
)

type NNTPServer struct {
	ctx        context.Context
	cancelFunc context.CancelFunc

	ln   net.Listener
	port int

	db *sqlx.DB

	sessionPool      map[string]*Session
	sessionPoolMutex sync.Mutex
}

func NewNNTPServer(cfg config.Config) (*NNTPServer, error) {
	db, err := sqlx.Open("sqlite3", cfg.DatabasePath)
	if err != nil {
		return nil, err
	}
	goose.SetBaseFS(internal.Migrations)

	if err := goose.SetDialect("sqlite3"); err != nil {
		return nil, err
	}

	if err := goose.Up(db.DB, "migrations"); err != nil {
		return nil, err
	}

	ctx, cancel := context.WithCancel(context.Background())
	ns := &NNTPServer{
		ctx:         ctx,
		cancelFunc:  cancel,
		port:        cfg.Port,
		db:          db,
		sessionPool: map[string]*Session{},
	}
	return ns, nil
}

func (ns *NNTPServer) Start() error {
	ln, err := net.Listen("tcp", fmt.Sprintf(":%d", ns.port))
	if err != nil {
		return err
	}

	go func(ctx context.Context) {
		for {
			select {
			case <-ctx.Done():
				break
			default:
				{
					conn, err := ln.Accept()
					if err != nil {
						log.Println(err)
					}
					log.Printf("Client %s has connected!", conn.RemoteAddr().String())

					id, _ := uuid.NewUUID()
					closed := make(chan bool)
					session, err := NewSession(ctx, conn, Capabilities, id.String(), closed, NewHandler(ns.db))
					ns.sessionPoolMutex.Lock()
					ns.sessionPool[id.String()] = session
					ns.sessionPoolMutex.Unlock()
					go func(ctx context.Context, id string, closed chan bool) {
						for {
							select {
							case <-ctx.Done():
								break
							case _, ok := <-closed:
								{
									if !ok {
										ns.sessionPoolMutex.Lock()
										delete(ns.sessionPool, id)
										ns.sessionPoolMutex.Unlock()
										return
									}
								}
							}
						}
					}(ctx, id.String(), closed)
				}
			}
		}
	}(ns.ctx)

	return nil
}

func (ns *NNTPServer) Stop() {
	ns.cancelFunc()
}
