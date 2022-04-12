package server

import (
	"context"
	"fmt"
	"github.com/ChronosX88/yans/internal/backend"
	"github.com/ChronosX88/yans/internal/backend/sqlite"
	"github.com/ChronosX88/yans/internal/common"
	"github.com/ChronosX88/yans/internal/config"
	"github.com/ChronosX88/yans/internal/protocol"
	"github.com/google/uuid"
	"log"
	"net"
	"sync"
)

var (
	Capabilities = protocol.Capabilities{
		{Type: protocol.VersionCapability, Params: "2"},
		{Type: protocol.ImplementationCapability, Params: fmt.Sprintf("%s %s", common.ServerName, common.ServerVersion)},
		{Type: protocol.OverCapability, Params: "MSGID"},
		{Type: protocol.ModeReaderCapability},
	}
)

type NNTPServer struct {
	ctx        context.Context
	cancelFunc context.CancelFunc

	ln  net.Listener
	cfg config.Config

	backend backend.StorageBackend

	sessionPool      map[string]*Session
	sessionPoolMutex sync.Mutex
}

func NewNNTPServer(cfg config.Config) (*NNTPServer, error) {
	b, err := initBackend(cfg)
	if err != nil {
		return nil, err
	}

	ctx, cancel := context.WithCancel(context.Background())
	ns := &NNTPServer{
		ctx:         ctx,
		cancelFunc:  cancel,
		cfg:         cfg,
		backend:     b,
		sessionPool: map[string]*Session{},
	}
	return ns, nil
}

func initBackend(cfg config.Config) (backend.StorageBackend, error) {
	var sb backend.StorageBackend

	switch cfg.BackendType {
	case config.SQLiteBackendType:
		{
			sqliteBackend, err := sqlite.NewSQLiteBackend(cfg.SQLite)
			if err != nil {
				return nil, err
			}
			sb = sqliteBackend
		}
	default:
		{
			return nil, fmt.Errorf("invalid backend type, supported backends: %s", backend.SupportedBackendList)
		}
	}
	return sb, nil
}

func (ns *NNTPServer) Start() error {
	address := fmt.Sprintf("%s:%d", ns.cfg.Address, ns.cfg.Port)
	ln, err := net.Listen("tcp", address)
	if err != nil {
		return err
	}

	log.Printf("Listening on %s...", address)

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
					session, err := NewSession(ctx, conn, Capabilities, id.String(), closed, NewHandler(ns.backend, ns.cfg.Domain, ns.cfg.UploadPath))
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
