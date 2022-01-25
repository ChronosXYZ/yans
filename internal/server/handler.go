package server

import (
	"database/sql"
	"fmt"
	"github.com/ChronosX88/yans/internal/backend"
	"github.com/ChronosX88/yans/internal/models"
	"github.com/ChronosX88/yans/internal/protocol"
	"strings"
	"time"
)

type Handler struct {
	handlers map[string]func(s *Session, arguments []string, id uint) error
	backend  backend.StorageBackend
}

func NewHandler(b backend.StorageBackend) *Handler {
	h := &Handler{}
	h.backend = b
	h.handlers = map[string]func(s *Session, arguments []string, id uint) error{
		protocol.CommandCapabilities: h.handleCapabilities,
		protocol.CommandDate:         h.handleDate,
		protocol.CommandQuit:         h.handleQuit,
		protocol.CommandList:         h.handleList,
		protocol.CommandMode:         h.handleModeReader,
		protocol.CommandGroup:        h.handleGroup,
		protocol.CommandNewGroups:    h.handleNewGroups,
	}
	return h
}

func (h *Handler) handleCapabilities(s *Session, arguments []string, id uint) error {
	s.tconn.StartResponse(id)
	defer s.tconn.EndResponse(id)
	return s.tconn.PrintfLine(s.capabilities.String())
}

func (h *Handler) handleDate(s *Session, arguments []string, id uint) error {
	s.tconn.StartResponse(id)
	defer s.tconn.EndResponse(id)
	return s.tconn.PrintfLine("111 %s", time.Now().UTC().Format("20060102150405"))
}

func (h *Handler) handleQuit(s *Session, arguments []string, id uint) error {
	s.tconn.PrintfLine(protocol.MessageNNTPServiceExitsNormally)
	s.conn.Close()
	return nil
}

func (h *Handler) handleList(s *Session, arguments []string, id uint) error {
	sb := strings.Builder{}

	listType := ""
	if len(arguments) != 0 {
		listType = arguments[0]
	}

	s.tconn.StartResponse(id)
	defer s.tconn.EndResponse(id)

	switch listType {
	case "":
		fallthrough
	case "ACTIVE":
		{
			var groups []models.Group
			var err error
			if len(arguments) == 2 {
				groups, err = h.backend.ListGroupsByPattern(arguments[1])
			} else {
				groups, err = h.backend.ListGroups()
			}

			if err != nil {
				return err
			}
			sb.Write([]byte(protocol.MessageListOfNewsgroupsFollows + protocol.CRLF))
			for _, v := range groups {
				// TODO set actual post permission status
				c, err := h.backend.GetArticlesCount(v)
				if err != nil {
					return err
				}
				if c > 0 {
					highWaterMark, err := h.backend.GetGroupHighWaterMark(v)
					if err != nil {
						return err
					}
					lowWaterMark, err := h.backend.GetGroupLowWaterMark(v)
					if err != nil {
						return err
					}
					sb.Write([]byte(fmt.Sprintf("%s %d %d n"+protocol.CRLF, v.GroupName, highWaterMark, lowWaterMark)))
				} else {
					sb.Write([]byte(fmt.Sprintf("%s 0 1 n"+protocol.CRLF, v.GroupName)))
				}
			}
		}
	case "NEWSGROUPS":
		{
			var groups []models.Group
			var err error
			if len(arguments) == 2 {
				groups, err = h.backend.ListGroupsByPattern(arguments[1])
			} else {
				groups, err = h.backend.ListGroups()
			}
			if err != nil {
				return err
			}

			sb.Write([]byte(protocol.MessageListOfNewsgroupsFollows + protocol.CRLF))
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
			return s.tconn.PrintfLine(protocol.MessageSyntaxError)
		}
	}

	sb.Write([]byte(protocol.MultilineEnding))

	return s.tconn.PrintfLine(sb.String())
}

func (h *Handler) handleModeReader(s *Session, arguments []string, id uint) error {
	if len(arguments) == 0 || arguments[0] != "READER" {
		return s.tconn.PrintfLine(protocol.MessageSyntaxError)
	}

	(&s.capabilities).Remove(protocol.ModeReaderCapability)
	(&s.capabilities).Remove(protocol.ListCapability)
	(&s.capabilities).Add(protocol.Capability{Type: protocol.ReaderCapability})
	(&s.capabilities).Add(protocol.Capability{Type: protocol.ListCapability, Params: "ACTIVE NEWSGROUPS"})
	s.mode = SessionModeReader

	return s.tconn.PrintfLine(protocol.MessageReaderModePostingProhibited) // TODO vary on auth status
}

func (h *Handler) handleGroup(s *Session, arguments []string, id uint) error {
	s.tconn.StartResponse(id)
	defer s.tconn.EndResponse(id)

	if len(arguments) == 0 || len(arguments) > 1 {
		return s.tconn.PrintfLine(protocol.MessageSyntaxError)
	}

	g, err := h.backend.GetGroup(arguments[0])
	if err != nil {
		if err == sql.ErrNoRows {
			return s.tconn.PrintfLine(protocol.MessageNoSuchGroup)
		} else {
			return err
		}
	}
	highWaterMark, err := h.backend.GetGroupHighWaterMark(g)
	if err != nil && err != sql.ErrNoRows {
		return err
	}
	lowWaterMark, err := h.backend.GetGroupLowWaterMark(g)
	if err != nil && err != sql.ErrNoRows {
		return err
	}
	articlesCount, err := h.backend.GetArticlesCount(g)
	if err != nil && err != sql.ErrNoRows {
		return err
	}

	s.currentGroup = &g

	return s.tconn.PrintfLine("211 %d %d %d %s", articlesCount, lowWaterMark, highWaterMark, g.GroupName)
}

func (h *Handler) handleNewGroups(s *Session, arguments []string, id uint) error {
	s.tconn.StartResponse(id)
	defer s.tconn.EndResponse(id)

	if len(arguments) < 2 || len(arguments) > 3 {
		return s.tconn.PrintfLine(protocol.MessageSyntaxError)
	}

	dateString := arguments[0] + " " + arguments[1]
	//isGMT := false
	//if len(arguments) == 3 {
	//	isGMT = true
	//}

	var date time.Time

	var err error
	if len(dateString) == 15 {
		date, err = time.Parse("20060102 150405", dateString)
		if err != nil {
			return err
		}
	} else if len(dateString) == 13 {
		date, err = time.Parse("060102 150405", dateString)
		if err != nil {
			return err
		}
	} else {
		return s.tconn.PrintfLine(protocol.MessageSyntaxError)
	}

	g, err := h.backend.GetNewGroupsSince(date.Unix())
	if err != nil {
		return err
	}

	var sb strings.Builder

	sb.Write([]byte(protocol.MessageListOfNewsgroupsFollows + protocol.CRLF))
	for _, v := range g {
		// TODO set actual post permission status
		c, err := h.backend.GetArticlesCount(v)
		if err != nil {
			return err
		}
		if c > 0 {
			highWaterMark, err := h.backend.GetGroupHighWaterMark(v)
			if err != nil {
				return err
			}
			lowWaterMark, err := h.backend.GetGroupLowWaterMark(v)
			if err != nil {
				return err
			}
			sb.Write([]byte(fmt.Sprintf("%s %d %d n"+protocol.CRLF, v.GroupName, highWaterMark, lowWaterMark)))
		} else {
			sb.Write([]byte(fmt.Sprintf("%s 0 1 n"+protocol.CRLF, v.GroupName)))
		}
	}
	sb.Write([]byte(protocol.MultilineEnding))

	return s.tconn.PrintfLine(sb.String())
}

func (h *Handler) Handle(s *Session, message string, id uint) error {
	splittedMessage := strings.Split(message, " ")
	for i, v := range splittedMessage {
		splittedMessage[i] = strings.TrimSpace(v)
	}
	cmdName := splittedMessage[0]
	handler, ok := h.handlers[cmdName]
	if !ok {
		s.tconn.StartResponse(id)
		defer s.tconn.EndResponse(id)
		return s.tconn.PrintfLine(protocol.MessageUnknownCommand)
	}
	return handler(s, splittedMessage[1:], id)
}
