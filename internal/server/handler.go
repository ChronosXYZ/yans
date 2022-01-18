package server

import (
	"fmt"
	"github.com/ChronosX88/yans/internal/models"
	"github.com/ChronosX88/yans/internal/protocol"
	"github.com/jmoiron/sqlx"
	"strings"
	"time"
)

type Handler struct {
	handlers map[string]func(s *Session, arguments []string) error
	db       *sqlx.DB
}

func NewHandler(db *sqlx.DB) *Handler {
	h := &Handler{}
	h.db = db
	h.handlers = map[string]func(s *Session, arguments []string) error{
		protocol.CommandCapabilities: h.handleCapabilities,
		protocol.CommandDate:         h.handleDate,
		protocol.CommandQuit:         h.handleQuit,
		protocol.CommandList:         h.handleList,
	}
	return h
}

func (h *Handler) handleCapabilities(s *Session, arguments []string) error {
	return s.tconn.PrintfLine(Capabilities.String())
}

func (h *Handler) handleDate(s *Session, arguments []string) error {
	return s.tconn.PrintfLine("111 %s", time.Now().UTC().Format("20060102150405"))
}

func (h *Handler) handleQuit(s *Session, arguments []string) error {
	s.tconn.PrintfLine(protocol.MessageNNTPServiceExitsNormally)
	s.conn.Close()
	return nil
}

func (h *Handler) handleList(s *Session, arguments []string) error {
	sb := strings.Builder{}

	listType := ""
	if len(arguments) != 0 {
		listType = arguments[0]
	}

	switch listType {
	case "":
		fallthrough
	case "ACTIVE":
		{
			groups, err := h.listGroups()
			if err != nil {
				return err
			}
			sb.Write([]byte(protocol.MessageListOfNewsgroupsFollows + protocol.CRLF))
			for _, v := range groups {
				// TODO set high/low mark and posting status to actual values
				sb.Write([]byte(fmt.Sprintf("%s 0 0 n"+protocol.CRLF, v.GroupName)))
			}
		}
	case "NEWSGROUPS":
		{
			groups, err := h.listGroups()
			if err != nil {
				return err
			}
			for _, v := range groups {
				desc := ""
				if v.Description == nil {
					desc = "No description"
				} else {
					desc = *v.Description
				}
				sb.Write([]byte(fmt.Sprintf("%s %s"+protocol.CRLF, v.GroupName, desc)))
			}
		}
	default:
		{
			return s.tconn.PrintfLine(protocol.MessageUnknownCommand)
		}
	}

	sb.Write([]byte(protocol.MultilineEnding))

	return s.tconn.PrintfLine(sb.String())
}

func (h *Handler) Handle(s *Session, message string) error {
	splittedMessage := strings.Split(message, " ")
	for i, v := range splittedMessage {
		splittedMessage[i] = strings.TrimSpace(v)
	}
	cmdName := splittedMessage[0]
	handler, ok := h.handlers[cmdName]
	if !ok {
		return s.tconn.PrintfLine(protocol.MessageUnknownCommand)
	}
	return handler(s, splittedMessage[1:])
}

// TODO Refactor to "storage backend" entity
func (h *Handler) listGroups() ([]models.Group, error) {
	var groups []models.Group
	return groups, h.db.Select(&groups, "SELECT * FROM groups")
}

func (h *Handler) getArticlesCount(g models.Group) (int, error) {
	var count int
	return count, h.db.Select(&count, "SELECT COUNT(*) FROM articles_to_groups WHERE group_id = ?", g.ID)
}
