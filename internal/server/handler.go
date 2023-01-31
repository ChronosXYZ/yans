package server

import (
	"bufio"
	"bytes"
	"database/sql"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net/mail"
	"path"
	"strconv"
	"strings"
	"time"

	"github.com/ChronosX88/yans/internal/backend"
	"github.com/ChronosX88/yans/internal/models"
	"github.com/ChronosX88/yans/internal/protocol"
	"github.com/ChronosX88/yans/internal/utils"
	"github.com/google/uuid"
	"github.com/jhillyerd/enmime"
)

type Handler struct {
	handlers     map[string]func(s *Session, command string, arguments []string, id uint) error
	backend      backend.StorageBackend
	serverDomain string
	uploadPath   string
}

func NewHandler(b backend.StorageBackend, serverDomain, uploadPath string) *Handler {
	h := &Handler{}
	h.backend = b
	h.handlers = map[string]func(s *Session, command string, arguments []string, id uint) error{
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
		protocol.CommandHead:         h.handleArticle,
		protocol.CommandBody:         h.handleArticle,
		protocol.CommandStat:         h.handleArticle,
		protocol.CommandHelp:         h.handleHelp,
		protocol.CommandNewNews:      h.handleNewNews,
		protocol.CommandLast:         h.handleLast,
		protocol.CommandNext:         h.handleNext,
		protocol.CommandOver:         h.handleOver,
		protocol.CommandXover:        h.handleOver,
		protocol.CommandIHave:        h.handleIHave,

		// project-specific extensions
		"NEWTHREADS": h.handleNewThreads,
		"THREAD":     h.handleThread,
	}
	h.serverDomain = serverDomain
	h.uploadPath = uploadPath
	return h
}

func (h *Handler) handleCapabilities(s *Session, command string, arguments []string, id uint) error {
	s.tconn.StartResponse(id)
	defer s.tconn.EndResponse(id)
	return s.tconn.PrintfLine(s.capabilities.String())
}

func (h *Handler) handleDate(s *Session, command string, arguments []string, id uint) error {
	s.tconn.StartResponse(id)
	defer s.tconn.EndResponse(id)
	return s.tconn.PrintfLine(protocol.NNTPResponse{Code: 111, Message: time.Now().UTC().Format("20060102150405")}.String())
}

func (h *Handler) handleQuit(s *Session, command string, arguments []string, id uint) error {
	s.tconn.PrintfLine(protocol.NNTPResponse{Code: 205, Message: "NNTP Service exits normally, bye!"}.String())
	s.conn.Close()
	return nil
}

func (h *Handler) handleList(s *Session, command string, arguments []string, id uint) error {
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
			dw := s.tconn.DotWriter()
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
			dw.Write([]byte(protocol.NNTPResponse{Code: 215, Message: "list of newsgroups follows"}.String() + protocol.CRLF))
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
					dw.Write([]byte(fmt.Sprintf("%s %d %d y"+protocol.CRLF, v.GroupName, highWaterMark, lowWaterMark)))
				} else {
					dw.Write([]byte(fmt.Sprintf("%s 0 1 y"+protocol.CRLF, v.GroupName)))
				}
			}
			return dw.Close()
		}
	case "NEWSGROUPS":
		{
			dw := s.tconn.DotWriter()
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

			dw.Write([]byte(protocol.NNTPResponse{Code: 215, Message: "list of newsgroups follows"}.String() + protocol.CRLF))
			for _, v := range groups {
				desc := ""
				if v.Description == nil {
					desc = "No description"
				} else {
					desc = *v.Description
				}
				dw.Write([]byte(fmt.Sprintf("%s %s"+protocol.CRLF, v.GroupName, desc)))
			}
			return dw.Close()
		}
	case "OVERVIEW.FMT":
		{
			dw := s.tconn.DotWriter()

			dw.Write([]byte(protocol.NNTPResponse{Code: 215, Message: "Order of fields in overview database."}.String() + protocol.CRLF))
			dw.Write([]byte("Subject:" + protocol.CRLF))
			dw.Write([]byte("From:" + protocol.CRLF))
			dw.Write([]byte("Date:" + protocol.CRLF))
			dw.Write([]byte("Message-ID:" + protocol.CRLF))
			dw.Write([]byte("References:" + protocol.CRLF))
			dw.Write([]byte(":bytes" + protocol.CRLF))
			dw.Write([]byte(":lines" + protocol.CRLF))

			return dw.Close()
		}
	default:
		{
			return s.tconn.PrintfLine(protocol.ErrSyntaxError.String())
		}
	}
}

func (h *Handler) handleModeReader(s *Session, command string, arguments []string, id uint) error {
	s.tconn.StartResponse(id)
	defer s.tconn.EndResponse(id)

	if len(arguments) == 0 || arguments[0] != "READER" {
		return s.tconn.PrintfLine(protocol.ErrSyntaxError.String())
	}

	(&s.capabilities).Remove(protocol.ModeReaderCapability)
	(&s.capabilities).Remove(protocol.ListCapability)
	(&s.capabilities).Add(protocol.Capability{Type: protocol.ReaderCapability})
	(&s.capabilities).Add(protocol.Capability{Type: protocol.ListCapability, Params: "ACTIVE NEWSGROUPS OVERVIEW.FMT"})
	s.mode = SessionModeReader

	return s.tconn.PrintfLine(protocol.NNTPResponse{Code: 201, Message: "Reader mode, posting prohibited"}.String()) // TODO vary on auth status
}

func (h *Handler) handleGroup(s *Session, command string, arguments []string, id uint) error {
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

	if lowWaterMark != 0 {
		a, err := h.backend.GetArticleByNumber(&g, lowWaterMark)
		if err != nil {
			return err
		}

		s.currentArticle = &a
	}

	return s.tconn.PrintfLine(protocol.NNTPResponse{
		Code:    211,
		Message: fmt.Sprintf("%d %d %d %s", articlesCount, lowWaterMark, highWaterMark, g.GroupName),
	}.String())
}

func (h *Handler) handleNewGroups(s *Session, command string, arguments []string, id uint) error {
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

func (h *Handler) handlePost(s *Session, command string, arguments []string, id uint) error {
	s.tconn.StartResponse(id)
	defer s.tconn.EndResponse(id)

	if len(arguments) != 0 {
		return s.tconn.PrintfLine(protocol.ErrSyntaxError.String())
	}

	if err := s.tconn.PrintfLine(protocol.NNTPResponse{Code: 340, Message: "Input article; end with <CR-LF>.<CR-LF>"}.String()); err != nil {
		return err
	}

	dr := s.tconn.DotReader()

	envelope, err := enmime.ReadEnvelope(dr)
	if err != nil {
		return err
	}

	h.saveArticle(envelope, true)
	if err != nil {
		if err.Error() == "no such message you are replying to" {
			return s.tconn.PrintfLine(protocol.NNTPResponse{Code: 441, Message: "no such message you are replying to"}.String())
		} else if err.Error() == "disallowed attachment type" {
			return s.tconn.PrintfLine(protocol.NNTPResponse{Code: 441, Message: "disallowed attachment type"}.String())
		}
		return s.tconn.PrintfLine(protocol.NNTPResponse{Code: 441, Message: err.Error()}.String())
	}

	return s.tconn.PrintfLine(protocol.NNTPResponse{Code: 240, Message: "Article received OK"}.String())
}

func (h *Handler) saveArticle(envelope *enmime.Envelope, generateHeaders bool) error {
	if generateHeaders {
		// generate message id
		messageID := fmt.Sprintf("<%s@%s>", uuid.New().String(), h.serverDomain)
		envelope.SetHeader("Message-ID", []string{messageID})

		// set path header
		envelope.SetHeader("Path", []string{fmt.Sprintf("%s!not-for-mail", h.serverDomain)})

		// set date header
		envelope.AddHeader("Date", time.Now().UTC().Format(time.RFC1123Z))
	}

	headerJson, err := json.Marshal(envelope.Root.Header)
	if err != nil {
		return err
	}

	a := models.Article{}
	a.HeaderRaw = string(headerJson)
	a.Header = envelope.Root.Header
	a.Envelope = envelope

	if err != nil {
		return err
	}
	a.Body = envelope.Text

	// set thread property
	if envelope.GetHeader("In-Reply-To") != "" {
		parentMessage, err := h.backend.GetArticle(envelope.GetHeader("In-Reply-To"))
		if err != nil {
			return err
			if err == sql.ErrNoRows {
				return fmt.Errorf("no such message you are replying to")
			} else {
				return err
			}
		}
		if !parentMessage.Thread.Valid {
			var parentHeader mail.Header
			err = json.Unmarshal([]byte(parentMessage.HeaderRaw), &parentHeader)
			parentMessageID := parentHeader.Get("Message-ID")
			a.Thread = sql.NullString{String: parentMessageID, Valid: true}
		} else {
			a.Thread = parentMessage.Thread
		}
	}

	if len(envelope.Attachments) > 0 {
		// save attachments
		for _, v := range envelope.Attachments {
			if v.ContentType != "image/jpeg" && v.ContentType != "image/png" && v.ContentType != "image/gif" {
				return fmt.Errorf("disallowed attachment type")
			}
			ext_ := strings.Split(v.FileName, ".")
			ext := ext_[len(ext_)-1]
			fileName := uuid.New().String() + "." + ext
			err = ioutil.WriteFile(path.Join(h.uploadPath, fileName), v.Content, 0644)
			if err != nil {
				return err
			}
			a.Attachments = append(a.Attachments, models.Attachment{
				ContentType: v.ContentType,
				FileName:    fileName,
			})
		}
	}

	err = h.backend.SaveArticle(a, strings.Split(a.Header.Get("Newsgroups"), ","))
	if err != nil {
		return err
	}

	return nil
}

func (h *Handler) handleListgroup(s *Session, command string, arguments []string, id uint) error {
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

func (h *Handler) handleArticle(s *Session, command string, arguments []string, id uint) error {
	s.tconn.StartResponse(id)
	defer s.tconn.EndResponse(id)

	var err error
	getByArticleNum := true
	if len(arguments) == 0 {
		if s.currentArticle == nil {
			return s.tconn.PrintfLine(protocol.NNTPResponse{Code: 420, Message: "No current article selected"}.String())
		}
		getByArticleNum = false
	}

	if len(arguments) > 1 {
		return s.tconn.PrintfLine(protocol.ErrSyntaxError.String())
	}

	var num int
	if getByArticleNum {
		num, err = strconv.Atoi(arguments[0])
		if err != nil {
			getByArticleNum = false
		}
	}

	if getByArticleNum && s.currentGroup == nil {
		return s.tconn.PrintfLine(protocol.NNTPResponse{Code: 412, Message: "No newsgroup selected"}.String())
	}

	var a *models.Article

	if getByArticleNum {
		article, err := h.backend.GetArticleByNumber(s.currentGroup, num)
		if err != nil {
			if err == sql.ErrNoRows {
				return s.tconn.PrintfLine(protocol.NNTPResponse{Code: 423, Message: "No article with that number"}.String())
			} else {
				return err
			}
		}
		a = &article
		s.currentArticle = &article
	} else if len(arguments) > 0 {
		article, err := h.backend.GetArticle(arguments[0])
		if err != nil {
			if err == sql.ErrNoRows {
				return s.tconn.PrintfLine(protocol.NNTPResponse{Code: 430, Message: "No Such Article Found"}.String())
			} else {
				return err
			}
		}
		a = &article
		s.currentArticle = &article
	} else {
		a = s.currentArticle
		num = s.currentArticle.ArticleNumber
	}

	switch command {
	case protocol.CommandArticle:
		{
			dw := s.tconn.DotWriter()
			builder := utils.Builder()
			_, err = dw.Write([]byte(protocol.NNTPResponse{Code: 220, Message: fmt.Sprintf("%d %s article", num, a.Header.Get("Message-ID"))}.String() + protocol.CRLF))
			for k, v := range a.Header {
				for _, j := range v {
					builder = builder.Header(k, j)
				}
			}
			builder = builder.Text([]byte(a.Body))
			for _, v := range a.Attachments {
				builder = builder.AddFileAttachment(path.Join(h.uploadPath, v.FileName))
			}
			p, err := builder.Build()
			if err != nil {
				return err
			}
			err = p.Encode(dw)
			if err != nil {
				return err
			}

			return dw.Close()
		}
	case protocol.CommandHead:
		{
			builder := utils.Builder()
			for k, v := range a.Header {
				for _, j := range v {
					builder = builder.Header(k, j)
				}
			}
			p, err := builder.Build()
			if err != nil {
				return err
			}

			dw := s.tconn.DotWriter()
			_, err = dw.Write([]byte(protocol.NNTPResponse{Code: 221, Message: fmt.Sprintf("%d %s", num, a.Header.Get("Message-ID"))}.String() + protocol.CRLF))
			if err != nil {
				return err
			}
			err = p.Encode(dw)
			if err != nil {
				return err
			}

			return dw.Close()
		}
	case protocol.CommandBody:
		{
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
	case protocol.CommandStat:
		{
			return s.tconn.PrintfLine(protocol.NNTPResponse{Code: 223, Message: fmt.Sprintf("%d %s", num, a.Header.Get("Message-ID"))}.String())
		}
	}

	return nil
}

func (h *Handler) handleHelp(s *Session, command string, arguments []string, id uint) error {
	s.tconn.StartResponse(id)
	defer s.tconn.EndResponse(id)

	help :=
		"  ARTICLE [message-ID|number]\r\n" +
			"  BODY [message-ID|number]\r\n" +
			"  CAPABILITIES [keyword]\r\n" +
			"  DATE\r\n" +
			"  GROUP newsgroup\r\n" +
			"  HEAD [message-ID|number]\r\n" +
			"  HELP\r\n" +
			"  LAST\r\n" +
			"  LIST [ACTIVE [wildmat]|NEWSGROUPS [wildmat]]\r\n" +
			"  LISTGROUP [newsgroup [range]]\r\n" +
			"  MODE READER\r\n" +
			"  NEWGROUPS [yy]yymmdd hhmmss [GMT]\r\n" +
			"  NEWNEWS [yy]yymmdd hhmmss [GMT]\r\n" +
			"  NEXT\r\n" +
			"  POST\r\n" +
			"  QUIT\r\n" +
			"  STAT [message-ID|number]\r\n"

	dw := s.tconn.DotWriter()
	w := bufio.NewWriter(dw)

	_, err := w.Write([]byte(protocol.NNTPResponse{Code: 100, Message: "Legal commands"}.String() + protocol.CRLF))
	if err != nil {
		return err
	}

	_, err = w.Write([]byte(help))
	if err != nil {
		return err
	}

	err = w.Flush()
	if err != nil {
		return err
	}

	return dw.Close()
}

func (h *Handler) handleNewNews(s *Session, command string, arguments []string, id uint) error {
	s.tconn.StartResponse(id)
	defer s.tconn.EndResponse(id)

	if len(arguments) < 2 || len(arguments) > 3 {
		return s.tconn.PrintfLine(protocol.ErrSyntaxError.String())
	}

	dateString := arguments[0] + " " + arguments[1]

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

	a, err := h.backend.GetNewArticlesSince(date.Unix())
	if err != nil {
		return err
	}

	dw := s.tconn.DotWriter()
	_, err = dw.Write([]byte(protocol.NNTPResponse{Code: 230, Message: "list of new articles by message-id follows"}.String() + protocol.CRLF))
	if err != nil {
		return err
	}
	for _, v := range a {
		_, err = dw.Write([]byte(v + protocol.CRLF))
		if err != nil {
			return err
		}
	}

	return dw.Close()
}

func (h *Handler) handleLast(s *Session, command string, arguments []string, id uint) error {
	s.tconn.StartResponse(id)
	defer s.tconn.EndResponse(id)

	if len(arguments) != 0 {
		return s.tconn.PrintfLine(protocol.ErrSyntaxError.String())
	}

	if s.currentGroup == nil {
		return s.tconn.PrintfLine(protocol.NNTPResponse{Code: 412, Message: "no newsgroup selected"}.String())
	}

	if s.currentArticle == nil {
		return s.tconn.PrintfLine(protocol.NNTPResponse{Code: 420, Message: "No current article selected"}.String())
	}

	low, err := h.backend.GetGroupLowWaterMark(s.currentGroup)
	if err != nil {
		return err
	}

	if s.currentArticle.ArticleNumber == low {
		return s.tconn.PrintfLine(protocol.NNTPResponse{Code: 422, Message: "No previous article to retrieve"}.String())
	}

	a, err := h.backend.GetLastArticleByNum(s.currentGroup, s.currentArticle)
	if err != nil {
		if err == sql.ErrNoRows {
			return s.tconn.PrintfLine(protocol.NNTPResponse{Code: 422, Message: "No previous article to retrieve"}.String())
		}
		return err
	}

	s.currentArticle = &a

	return s.tconn.PrintfLine(protocol.NNTPResponse{Code: 223, Message: fmt.Sprintf("%d %s retrieved", a.ArticleNumber, a.Header.Get("Message-ID"))}.String())
}

func (h *Handler) handleNext(s *Session, command string, arguments []string, id uint) error {
	s.tconn.StartResponse(id)
	defer s.tconn.EndResponse(id)

	if len(arguments) != 0 {
		return s.tconn.PrintfLine(protocol.ErrSyntaxError.String())
	}

	if s.currentGroup == nil {
		return s.tconn.PrintfLine(protocol.NNTPResponse{Code: 412, Message: "no newsgroup selected"}.String())
	}

	if s.currentArticle == nil {
		return s.tconn.PrintfLine(protocol.NNTPResponse{Code: 420, Message: "No current article selected"}.String())
	}

	high, err := h.backend.GetGroupHighWaterMark(s.currentGroup)
	if err != nil {
		return err
	}

	if s.currentArticle.ArticleNumber == high {
		return s.tconn.PrintfLine(protocol.NNTPResponse{Code: 421, Message: "No next article to retrieve"}.String())
	}

	a, err := h.backend.GetNextArticleByNum(s.currentGroup, s.currentArticle)
	if err != nil {
		if err == sql.ErrNoRows {
			return s.tconn.PrintfLine(protocol.NNTPResponse{Code: 421, Message: "No next article to retrieve"}.String())
		}
		return err
	}

	s.currentArticle = &a

	return s.tconn.PrintfLine(protocol.NNTPResponse{Code: 223, Message: fmt.Sprintf("%d %s retrieved", a.ArticleNumber, a.Header.Get("Message-Id"))}.String())
}

func (h *Handler) handleOver(s *Session, command string, arguments []string, id uint) error {
	s.tconn.StartResponse(id)
	defer s.tconn.EndResponse(id)

	if len(arguments) == 0 && s.currentArticle == nil {
		return s.tconn.PrintfLine(protocol.NNTPResponse{Code: 420, Message: "No current article selected"}.String())
	}

	byRange := false
	byNum := false
	byMsgID := false
	curArticle := false

	if len(arguments) == 1 {
		if _, _, err := utils.ParseRange(arguments[0]); err == nil {
			byRange = true
		} else if strings.ContainsAny(arguments[0], "<>") {
			byMsgID = true
		} else if _, err := strconv.Atoi(arguments[0]); err == nil {
			byNum = true
		}
	} else if len(arguments) == 0 {
		curArticle = true
	} else {
		return s.tconn.PrintfLine(protocol.ErrSyntaxError.String())
	}

	var articles []models.Article

	if byRange {
		if s.currentGroup == nil {
			return s.tconn.PrintfLine(protocol.NNTPResponse{Code: 412, Message: "No newsgroup selected"}.String())
		}

		low, high, err := utils.ParseRange(arguments[0])
		if err != nil {
			return err
		}
		if low > high {
			return s.tconn.PrintfLine(protocol.NNTPResponse{Code: 423, Message: "Empty range"}.String())
		}
		a, err := h.backend.GetArticlesByRange(s.currentGroup, low, high)
		if err != nil {
			if err == sql.ErrNoRows {
				return s.tconn.PrintfLine(protocol.NNTPResponse{Code: 423, Message: "No articles in that range"}.String())
			}
			return err
		}
		articles = append(articles, a...)
	} else if byMsgID {
		a, err := h.backend.GetArticle(arguments[0])
		if err != nil {
			if err == sql.ErrNoRows {
				return s.tconn.PrintfLine(protocol.NNTPResponse{Code: 430, Message: "No such article with that message-id"}.String())
			}
			return err
		}
		a.ArticleNumber = 0
		articles = append(articles, a)
	} else if byNum {
		num, _ := strconv.Atoi(arguments[0])
		a, err := h.backend.GetArticleByNumber(s.currentGroup, num)
		if err != nil {
			if err == sql.ErrNoRows {
				return s.tconn.PrintfLine(protocol.NNTPResponse{Code: 423, Message: "No such article in this group"}.String())
			}
			return err
		}
		articles = append(articles, a)
	} else if curArticle {
		articles = append(articles, *s.currentArticle)
	}

	dw := s.tconn.DotWriter()
	dw.Write([]byte(protocol.NNTPResponse{Code: 224, Message: "Overview information follows" + protocol.CRLF}.String()))
	for _, v := range articles {
		dw.Write([]byte(strconv.Itoa(v.ArticleNumber) + "	"))
		dw.Write([]byte(v.Header.Get("Subject") + "	"))
		dw.Write([]byte(v.Header.Get("From") + "	"))
		dw.Write([]byte(v.Header.Get("Date") + "	"))
		dw.Write([]byte(v.Header.Get("Message-ID") + "	"))
		dw.Write([]byte(v.Header.Get("References") + "	"))

		// count bytes for message
		builder := utils.Builder()
		for k, v := range v.Header {
			for _, j := range v {
				builder = builder.Header(k, j)
			}
		}
		builder = builder.Text([]byte(v.Body)) // FIXME currently only plain text is supported
		b := bytes.NewBuffer([]byte{})
		p, err := builder.Build()
		if err != nil {
			return err
		}
		err = p.Encode(b)
		if err != nil {
			return err
		}

		bytesMetadata := b.Len()
		linesMetadata := strings.Count(v.Body, "\n")

		dw.Write([]byte(strconv.Itoa(bytesMetadata) + "	"))
		dw.Write([]byte(strconv.Itoa(linesMetadata) + protocol.CRLF))
	}

	return dw.Close()
}

func (h *Handler) handleNewThreads(s *Session, command string, arguments []string, id uint) error {
	s.tconn.StartResponse(id)
	defer s.tconn.EndResponse(id)

	if s.currentGroup == nil {
		return s.tconn.PrintfLine(protocol.NNTPResponse{Code: 412, Message: "no newsgroup selected"}.String())
	}

	if len(arguments) == 0 {
		return s.tconn.PrintfLine(protocol.ErrSyntaxError.String())
	}

	perPage, err := strconv.Atoi(arguments[0])
	if err != nil {
		return s.tconn.PrintfLine(protocol.ErrSyntaxError.String())
	}
	pageNum, err := strconv.Atoi(arguments[1])
	if err != nil {
		return s.tconn.PrintfLine(protocol.ErrSyntaxError.String())
	}

	threadNums, err := h.backend.GetNewThreads(s.currentGroup, perPage, pageNum)
	if err != nil {
		if err != sql.ErrNoRows {
			return err
		}
	}

	dw := s.tconn.DotWriter()
	dw.Write([]byte(protocol.NNTPResponse{Code: 225, Message: "New thread numbers follows" + protocol.CRLF}.String()))
	for _, v := range threadNums {
		dw.Write([]byte(strconv.Itoa(v) + protocol.CRLF))
	}
	return dw.Close()
}

func (h *Handler) handleThread(s *Session, command string, arguments []string, id uint) error {
	s.tconn.StartResponse(id)
	defer s.tconn.EndResponse(id)

	if s.currentGroup == nil {
		return s.tconn.PrintfLine(protocol.NNTPResponse{Code: 412, Message: "no newsgroup selected"}.String())
	}

	if len(arguments) == 0 {
		return s.tconn.PrintfLine(protocol.ErrSyntaxError.String())
	}

	threadNumber, err := strconv.Atoi(arguments[0])
	if err != nil {
		return s.tconn.PrintfLine(protocol.ErrSyntaxError.String())
	}

	threadNums, err := h.backend.GetThread(s.currentGroup, threadNumber)
	if err != nil {
		if err != sql.ErrNoRows {
			return err
		}
	}

	dw := s.tconn.DotWriter()
	dw.Write([]byte(protocol.NNTPResponse{Code: 226, Message: "Thread articles follows" + protocol.CRLF}.String()))
	for _, v := range threadNums {
		dw.Write([]byte(strconv.Itoa(v) + protocol.CRLF))
	}
	return dw.Close()
}

func (h *Handler) handleIHave(s *Session, command string, arguments []string, id uint) error {
	s.tconn.StartResponse(id)
	defer s.tconn.EndResponse(id)

	if len(arguments) != 1 {
		return s.tconn.PrintfLine(protocol.ErrSyntaxError.String())
	}

	if _, err := h.backend.GetArticle(arguments[0]); err == nil {
		return s.tconn.PrintfLine(protocol.NNTPResponse{Code: 435, Message: "Duplicate"}.String())
	} else {
		log.Print(err.Error())
	}

	if err := s.tconn.PrintfLine(protocol.NNTPResponse{Code: 335, Message: "Send it; end with <CR-LF>.<CR-LF>"}.String()); err != nil {
		return err
	}
	// TODO restrict sending the same article from other users

	dr := s.tconn.DotReader()

	envelope, err := enmime.ReadEnvelope(dr)
	if err != nil {
		return err
	}

	msgID := envelope.GetHeader("Message-ID")
	if msgID == "" {
		return s.tconn.PrintfLine(protocol.NNTPResponse{Code: 436, Message: "Transfer failed"}.String())
	}
	// TODO also check whether message id in the article is the same as was previously

	err = h.saveArticle(envelope, false)
	if err != nil {
		// TODO add proper error handling
		return s.tconn.PrintfLine(protocol.NNTPResponse{Code: 436, Message: fmt.Sprintf("Transfer failed: %s", err.Error())}.String())
	}

	return s.tconn.PrintfLine(protocol.NNTPResponse{Code: 235, Message: "Article transferred OK"}.String())
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
	return handler(s, cmdName, splittedMessage[1:], id)
}
