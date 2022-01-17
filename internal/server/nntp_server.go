package server

import (
	"bufio"
	"context"
	"fmt"
	"github.com/ChronosX88/yans/internal"
	"github.com/ChronosX88/yans/internal/config"
	"github.com/ChronosX88/yans/internal/models"
	"github.com/ChronosX88/yans/internal/protocol"
	"github.com/jmoiron/sqlx"
	_ "github.com/mattn/go-sqlite3"
	"github.com/pressly/goose/v3"
	"io"
	"log"
	"net"
	"strings"
	"time"
)

type NNTPServer struct {
	ctx        context.Context
	cancelFunc context.CancelFunc

	ln   net.Listener
	port int

	db *sqlx.DB
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
		ctx:        ctx,
		cancelFunc: cancel,
		port:       cfg.Port,
		db:         db,
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
					go ns.handleNewConnection(ctx, conn)
				}
			}
		}
	}(ns.ctx)

	return nil
}

func (ns *NNTPServer) handleNewConnection(ctx context.Context, conn net.Conn) {
	_, err := conn.Write([]byte(protocol.MessageNNTPServiceReadyPostingProhibited))
	if err != nil {
		log.Print(err)
		conn.Close()
		return
	}
	for {
		select {
		case <-ctx.Done():
			break
		default:
			{
				message, err := bufio.NewReader(conn).ReadString('\n')
				if err != nil {
					if err == io.EOF || err.(*net.OpError).Unwrap() == net.ErrClosed {
						log.Printf("Client %s has diconnected!", conn.RemoteAddr().String())
					} else {
						log.Print(err)
						conn.Close()
					}
					return
				}
				log.Printf("Received message from %s: %s", conn.RemoteAddr().String(), string(message))
				err = ns.handleMessage(conn, message)
				if err != nil {
					log.Print(err)
					conn.Close()
					return
				}
			}
		}
	}
}

func (ns *NNTPServer) handleMessage(conn net.Conn, msg string) error {
	msg = strings.TrimSuffix(msg, "\r\n")
	splittedMessage := strings.Split(msg, " ")
	command := splittedMessage[0]

	reply := ""
	quit := false

	switch command {
	case protocol.CommandCapabilities:
		{
			reply = "101 Capability list:\r\nVERSION 2\r\nIMPLEMENTATION\r\n."
			break
		}
	case protocol.CommandDate:
		{
			reply = fmt.Sprintf("111 %s", time.Now().UTC().Format("20060102150405"))
			break
		}
	case protocol.CommandQuit:
		{
			reply = protocol.MessageNNTPServiceExitsNormally
			quit = true
			break
		}
	case protocol.CommandMode:
		{
			if splittedMessage[1] == "READER" {
				// TODO actually switch current conn to reader mode
				reply = protocol.MessageReaderModePostingProhibited
			} else {
				reply = protocol.MessageUnknownCommand
			}
			break
		}
	case protocol.CommandList:
		{
			groups, err := ns.listGroups()
			if err != nil {
				reply = protocol.MessageErrorHappened + err.Error()
				log.Println(err)
			}
			sb := strings.Builder{}
			sb.Write([]byte("215 list of newsgroups follows\n"))
			if len(splittedMessage) == 1 || splittedMessage[1] == "ACTIVE" {
				for _, v := range groups {
					// TODO set high/low mark and posting status to actual values
					sb.Write([]byte(fmt.Sprintf("%s 0 0 n\r\n", v.GroupName)))
				}
			} else if splittedMessage[1] == "NEWSGROUPS" {
				for _, v := range groups {
					desc := ""
					if v.Description == nil {
						desc = "No description"
					} else {
						desc = *v.Description
					}
					sb.Write([]byte(fmt.Sprintf("%s %s\r\n", v.GroupName, desc)))
				}
			} else {
				reply = protocol.MessageUnknownCommand
				break
			}

			sb.Write([]byte("."))
			reply = sb.String()
		}
	default:
		{
			reply = protocol.MessageUnknownCommand
			break
		}
	}

	_, err := conn.Write([]byte(reply + "\r\n"))
	if quit {
		conn.Close()
	}
	return err
}

func (ns *NNTPServer) listGroups() ([]models.Group, error) {
	var groups []models.Group
	return groups, ns.db.Select(&groups, "SELECT * FROM groups")
}

func (ns *NNTPServer) getArticlesCount(g models.Group) (int, error) {
	var count int
	return count, ns.db.Select(&count, "SELECT COUNT(*) FROM articles_to_groups WHERE group_id = ?", g.ID)
}

func (ns *NNTPServer) Stop() {
	ns.cancelFunc()
}
