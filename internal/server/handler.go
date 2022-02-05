package server

import (
	"bufio"
	"database/sql"
	"encoding/json"
	"fmt"
	"github.com/ChronosX88/yans/internal/backend"
	"github.com/ChronosX88/yans/internal/models"
	"github.com/ChronosX88/yans/internal/protocol"
	"github.com/ChronosX88/yans/internal/utils"
	"github.com/google/uuid"
	"io"
	"net/mail"
	"strconv"
	"strings"
	"time"
)

type Handler struct {
	handlers     map[string]func(s *Session, arguments []string, id uint) error
	backend      backend.StorageBackend
	serverDomain string
}

func NewHandler(b backend.StorageBackend, serverDomain string) *Handler {
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
		protocol.CommandPost:         h.handlePost,
		protocol.CommandListGroup:    h.handleListgroup,
		protocol.CommandArticle:      h.handleArticle,
		protocol.CommandHead:         h.handleHead,
		protocol.CommandBody:         h.handleBody,
	}
	h.serverDomain = serverDomain
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
	return s.tconn.PrintfLine(protocol.NNTPResponse{Code: 111, Message: time.Now().UTC().Format("20060102150405")}.String())
}

func (h *Handler) handleQuit(s *Session, arguments []string, id uint) error {
	s.tconn.PrintfLine(protocol.NNTPResponse{Code: 205, Message: "NNTP Service exits normally, bye!"}.String())
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
			sb.Write([]byte(protocol.NNTPResponse{Code: 215, Message: "list of newsgroups follows"}.String() + protocol.CRLF))
			for _, v := range groups {
				// TODO set actual post permission status
				c, err := h.backend.GetArticlesCount(&v)
				if err != nil {
					return err
				}
				if c > 0 {
					highWaterMark, err := h.backend.GetGroupHighWaterMark(&v)
					if err != nil {
						return err
					}
					lowWaterMark, err := h.backend.GetGroupLowWaterMark(&v)
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

			sb.Write([]byte(protocol.NNTPResponse{Code: 215, Message: "list of newsgroups follows"}.String() + protocol.CRLF))
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
			return s.tconn.PrintfLine(protocol.ErrSyntaxError.String())
		}
	}

	sb.Write([]byte(protocol.MultilineEnding))

	return s.tconn.PrintfLine(sb.String())
}

func (h *Handler) handleModeReader(s *Session, arguments []string, id uint) error {
	s.tconn.StartResponse(id)
	defer s.tconn.EndResponse(id)

	if len(arguments) == 0 || arguments[0] != "READER" {
		return s.tconn.PrintfLine(protocol.ErrSyntaxError.String())
	}

	(&s.capabilities).Remove(protocol.ModeReaderCapability)
	(&s.capabilities).Remove(protocol.ListCapability)
	(&s.capabilities).Add(protocol.Capability{Type: protocol.ReaderCapability})
	(&s.capabilities).Add(protocol.Capability{Type: protocol.ListCapability, Params: "ACTIVE NEWSGROUPS"})
	s.mode = SessionModeReader

	return s.tconn.PrintfLine(protocol.NNTPResponse{Code: 201, Message: "Reader mode, posting prohibited"}.String()) // TODO vary on auth status
}

func (h *Handler) handleGroup(s *Session, arguments []string, id uint) error {
	s.tconn.StartResponse(id)
	defer s.tconn.EndResponse(id)

	if len(arguments) == 0 || len(arguments) > 1 {
		return s.tconn.PrintfLine(protocol.ErrSyntaxError.String())
	}

	g, err := h.backend.GetGroup(arguments[0])
	if err != nil {
		if err == sql.ErrNoRows {
			return s.tconn.PrintfLine(protocol.NNTPResponse{Code: 411, Message: "No such newsgroup"}.String())
		} else {
			return err
		}
	}
	highWaterMark, err := h.backend.GetGroupHighWaterMark(&g)
	if err != nil && err != sql.ErrNoRows {
		return err
	}
	lowWaterMark, err := h.backend.GetGroupLowWaterMark(&g)
	if err != nil && err != sql.ErrNoRows {
		return err
	}
	articlesCount, err := h.backend.GetArticlesCount(&g)
	if err != nil && err != sql.ErrNoRows {
		return err
	}

	s.currentGroup = &g

	return s.tconn.PrintfLine(protocol.NNTPResponse{
		Code:    211,
		Message: fmt.Sprintf("%d %d %d %s", articlesCount, lowWaterMark, highWaterMark, g.GroupName),
	}.String())
}

func (h *Handler) handleNewGroups(s *Session, arguments []string, id uint) error {
	s.tconn.StartResponse(id)
	defer s.tconn.EndResponse(id)

	if len(arguments) < 2 || len(arguments) > 3 {
		return s.tconn.PrintfLine(protocol.ErrSyntaxError.String())
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
		return s.tconn.PrintfLine(protocol.ErrSyntaxError.String())
	}

	g, err := h.backend.GetNewGroupsSince(date.Unix())
	if err != nil {
		return err
	}

	dw := s.tconn.DotWriter()
	dw.Write([]byte(protocol.NNTPResponse{Code: 231, Message: "list of new newsgroups follows"}.String() + protocol.CRLF))
	for _, v := range g {
		// TODO set actual post permission status
		c, err := h.backend.GetArticlesCount(&v)
		if err != nil {
			return err
		}
		if c > 0 {
			highWaterMark, err := h.backend.GetGroupHighWaterMark(&v)
			if err != nil {
				return err
			}
			lowWaterMark, err := h.backend.GetGroupLowWaterMark(&v)
			if err != nil {
				return err
			}
			dw.Write([]byte(fmt.Sprintf("%s %d %d n"+protocol.CRLF, v.GroupName, highWaterMark, lowWaterMark)))
		} else {
			dw.Write([]byte(fmt.Sprintf("%s 0 1 n"+protocol.CRLF, v.GroupName)))
		}
	}

	return dw.Close()
}

func (h *Handler) handlePost(s *Session, arguments []string, id uint) error {
	s.tconn.StartResponse(id)
	defer s.tconn.EndResponse(id)

	if len(arguments) != 0 {
		return s.tconn.PrintfLine(protocol.ErrSyntaxError.String())
	}

	if err := s.tconn.PrintfLine(protocol.NNTPResponse{Code: 340, Message: "Input article; end with <CR-LF>.<CR-LF>"}.String()); err != nil {
		return err
	}

	headers, err := s.tconn.ReadMIMEHeader()
	if err != nil {
		return err
	}

	// generate message id
	messageID := fmt.Sprintf("<%s@%s>", uuid.New().String(), h.serverDomain)
	headers.Set("Message-ID", messageID)

	headerJson, err := json.Marshal(headers)
	if err != nil {
		return err
	}

	a := models.Article{}
	a.HeaderRaw = string(headerJson)
	a.Header = headers

	dr := s.tconn.DotReader()
	// TODO handle multipart message
	body, err := io.ReadAll(dr)
	if err != nil {
		return err
	}
	a.Body = string(body)

	// set thread property
	if headers.Get("In-Reply-To") != "" {
		parentMessage, err := h.backend.GetArticle(headers.Get("In-Reply-To"))
		if err != nil {
			if err == sql.ErrNoRows {
				return s.tconn.PrintfLine(protocol.NNTPResponse{Code: 441, Message: "no such message you are replying to"}.String())
			} else {
				return err
			}
		}
		if !parentMessage.Thread.Valid {
			var parentHeader mail.Header
			err = json.Unmarshal([]byte(parentMessage.HeaderRaw), &parentHeader)
			parentMessageID := parentHeader["Message-ID"]
			a.Thread = sql.NullString{String: parentMessageID[0], Valid: true}
		} else {
			a.Thread = parentMessage.Thread
		}
	}

	err = h.backend.SaveArticle(a, strings.Split(a.Header.Get("Newsgroups"), ","))
	if err != nil {
		return s.tconn.PrintfLine(protocol.NNTPResponse{Code: 441, Message: err.Error()}.String())
	}

	return s.tconn.PrintfLine(protocol.NNTPResponse{Code: 240, Message: "Article received OK"}.String())
}

func (h *Handler) handleListgroup(s *Session, arguments []string, id uint) error {
	s.tconn.StartResponse(id)
	defer s.tconn.EndResponse(id)

	currentGroup := s.currentGroup
	var low, high int64
	if len(arguments) == 1 {
		g, err := h.backend.GetGroup(arguments[0])
		if err != nil {
			return s.tconn.PrintfLine(protocol.NNTPResponse{Code: 411, Message: "No such newsgroup"}.String())
		}
		currentGroup = &g
	} else if len(arguments) == 2 {
		g, err := h.backend.GetGroup(arguments[0])
		if err != nil {
			return s.tconn.PrintfLine(protocol.NNTPResponse{Code: 411, Message: "No such newsgroup"}.String())
		}
		currentGroup = &g

		low, high, err = utils.ParseRange(arguments[1])
		if err != nil {
			low = 0
			high = 0
		}
		if high != -1 && low > high {
			low = -1
			high = -1
		}
	}

	if currentGroup == nil {
		return s.tconn.PrintfLine(protocol.NNTPResponse{Code: 412, Message: "No newsgroup selected"}.String())
	}

	highWaterMark, err := h.backend.GetGroupHighWaterMark(currentGroup)
	if err != nil && err != sql.ErrNoRows {
		return err
	}
	lowWaterMark, err := h.backend.GetGroupLowWaterMark(currentGroup)
	if err != nil && err != sql.ErrNoRows {
		return err
	}
	articlesCount, err := h.backend.GetArticlesCount(currentGroup)
	if err != nil && err != sql.ErrNoRows {
		return err
	}

	nums, err := h.backend.GetArticleNumbers(currentGroup, low, high)
	if err != nil && err != sql.ErrNoRows {
		return err
	}

	dw := s.tconn.DotWriter()
	dw.Write([]byte(protocol.NNTPResponse{Code: 211, Message: fmt.Sprintf("%d %d %d %s list follows%s", articlesCount, lowWaterMark, highWaterMark, currentGroup.GroupName, protocol.CRLF)}.String()))
	for _, v := range nums {
		dw.Write([]byte(strconv.FormatInt(v, 10) + protocol.CRLF))
	}
	return dw.Close()
}

func (h *Handler) handleArticle(s *Session, arguments []string, id uint) error {
	s.tconn.StartResponse(id)
	defer s.tconn.EndResponse(id)

	if len(arguments) == 0 && s.currentArticle == nil {
		return s.tconn.PrintfLine(protocol.NNTPResponse{Code: 420, Message: "No current article selected"}.String())
	}

	if len(arguments) > 1 {
		return s.tconn.PrintfLine(protocol.ErrSyntaxError.String())
	}

	getByArticleNum := true
	num, err := strconv.Atoi(arguments[0])
	if err != nil {
		getByArticleNum = false
	}

	if getByArticleNum && s.currentGroup == nil {
		return s.tconn.PrintfLine(protocol.NNTPResponse{Code: 412, Message: "No newsgroup selected"}.String())
	}

	var a models.Article

	if getByArticleNum {
		a, err = h.backend.GetArticleByNumber(s.currentGroup, num)
		if err != nil {
			if err == sql.ErrNoRows {
				return s.tconn.PrintfLine(protocol.NNTPResponse{Code: 423, Message: "No article with that number"}.String())
			} else {
				return err
			}
		}
	} else {
		a, err = h.backend.GetArticle(arguments[0])
		if err != nil {
			if err == sql.ErrNoRows {
				return s.tconn.PrintfLine(protocol.NNTPResponse{Code: 430, Message: "No Such Article Found"}.String())
			} else {
				return err
			}
		}
	}

	s.currentArticle = &a

	err = json.Unmarshal([]byte(a.HeaderRaw), &a.Header)
	if err != nil {
		return err
	}

	m := utils.NewMessage()
	for k, v := range a.Header {
		m.SetHeader(k, v...)
	}

	m.SetBody("text/plain", a.Body) // FIXME currently only plain text is supported
	dw := s.tconn.DotWriter()
	_, err = dw.Write([]byte(protocol.NNTPResponse{Code: 220, Message: fmt.Sprintf("%d %s", num, a.Header.Get("Message-ID"))}.String() + protocol.CRLF))
	if err != nil {
		return err
	}
	_, err = m.WriteTo(dw)
	if err != nil {
		return err
	}

	return dw.Close()
}

// FIXME refactor this, because it's mostly duplicate of ARTICLE handler function
func (h *Handler) handleHead(s *Session, arguments []string, id uint) error {
	s.tconn.StartResponse(id)
	defer s.tconn.EndResponse(id)

	if len(arguments) == 0 && s.currentArticle == nil {
		return s.tconn.PrintfLine(protocol.NNTPResponse{Code: 420, Message: "No current article selected"}.String())
	}

	if len(arguments) > 1 {
		return s.tconn.PrintfLine(protocol.ErrSyntaxError.String())
	}

	getByArticleNum := true
	num, err := strconv.Atoi(arguments[0])
	if err != nil {
		getByArticleNum = false
	}

	if getByArticleNum && s.currentGroup == nil {
		return s.tconn.PrintfLine(protocol.NNTPResponse{Code: 412, Message: "No newsgroup selected"}.String())
	}

	var a models.Article

	if getByArticleNum {
		a, err = h.backend.GetArticleByNumber(s.currentGroup, num)
		if err != nil {
			if err == sql.ErrNoRows {
				return s.tconn.PrintfLine(protocol.NNTPResponse{Code: 423, Message: "No article with that number"}.String())
			} else {
				return err
			}
		}
	} else {
		a, err = h.backend.GetArticle(arguments[0])
		if err != nil {
			if err == sql.ErrNoRows {
				return s.tconn.PrintfLine(protocol.NNTPResponse{Code: 430, Message: "No Such Article Found"}.String())
			} else {
				return err
			}
		}
	}

	s.currentArticle = &a

	m := utils.NewMessage()
	for k, v := range a.Header {
		m.SetHeader(k, v...)
	}

	dw := s.tconn.DotWriter()
	_, err = dw.Write([]byte(protocol.NNTPResponse{Code: 221, Message: fmt.Sprintf("%d %s", num, a.Header.Get("Message-ID"))}.String() + protocol.CRLF))
	if err != nil {
		return err
	}
	_, err = m.WriteTo(dw)
	if err != nil {
		return err
	}

	return dw.Close()
}

// FIXME refactor this, because it's mostly duplicate of ARTICLE handler function
func (h *Handler) handleBody(s *Session, arguments []string, id uint) error {
	s.tconn.StartResponse(id)
	defer s.tconn.EndResponse(id)

	if len(arguments) == 0 && s.currentArticle == nil {
		return s.tconn.PrintfLine(protocol.NNTPResponse{Code: 420, Message: "No current article selected"}.String())
	}

	if len(arguments) > 1 {
		return s.tconn.PrintfLine(protocol.ErrSyntaxError.String())
	}

	getByArticleNum := true
	num, err := strconv.Atoi(arguments[0])
	if err != nil {
		getByArticleNum = false
	}

	if getByArticleNum && s.currentGroup == nil {
		return s.tconn.PrintfLine(protocol.NNTPResponse{Code: 412, Message: "No newsgroup selected"}.String())
	}

	var a models.Article

	if getByArticleNum {
		a, err = h.backend.GetArticleByNumber(s.currentGroup, num)
		if err != nil {
			if err == sql.ErrNoRows {
				return s.tconn.PrintfLine(protocol.NNTPResponse{Code: 423, Message: "No article with that number"}.String())
			} else {
				return err
			}
		}
	} else {
		a, err = h.backend.GetArticle(arguments[0])
		if err != nil {
			if err == sql.ErrNoRows {
				return s.tconn.PrintfLine(protocol.NNTPResponse{Code: 430, Message: "No Such Article Found"}.String())
			} else {
				return err
			}
		}
	}

	s.currentArticle = &a

	dw := s.tconn.DotWriter()
	
	_, err = dw.Write([]byte(protocol.NNTPResponse{Code: 222, Message: fmt.Sprintf("%d %s", num, a.Header.Get("Message-ID"))}.String() + protocol.CRLF))
	if err != nil {
		return err
	}

	w := bufio.NewWriter(dw)
	_, err = w.Write([]byte(a.Body))
	if err != nil {
		return err
	}

	err = w.Flush()
	if err != nil {
		return err
	}

	return dw.Close()
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
		return s.tconn.PrintfLine(protocol.NNTPResponse{Code: 500, Message: "Unknown command"}.String())
	}
	return handler(s, splittedMessage[1:], id)
}
