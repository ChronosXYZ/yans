package utils

// this package has been kindly taken from https://github.com/go-gomail/gomail
// licensed under MIT license

// Copyright (c) 2014 Alexandre Cesaro

import (
	"bytes"
	"encoding/base64"
	"io"
	"mime"
	"mime/multipart"
	"mime/quotedprintable"
	"os"
	"path/filepath"
	"strings"
	"time"
)

var newQPWriter = quotedprintable.NewWriter

type mimeEncoder struct {
	mime.WordEncoder
}

var (
	bEncoding     = mimeEncoder{mime.BEncoding}
	qEncoding     = mimeEncoder{mime.QEncoding}
	lastIndexByte = strings.LastIndexByte
)

// Message represents an email.
type Message struct {
	header      header
	parts       []*part
	attachments []*file
	embedded    []*file
	charset     string
	encoding    Encoding
	hEncoder    mimeEncoder
	buf         bytes.Buffer
}

type header map[string][]string

type part struct {
	contentType string
	copier      func(io.Writer) error
	encoding    Encoding
}

// NewMessage creates a new message. It uses UTF-8 and quoted-printable encoding
// by default.
func NewMessage(settings ...MessageSetting) *Message {
	m := &Message{
		header:   make(header),
		charset:  "UTF-8",
		encoding: QuotedPrintable,
	}

	m.applySettings(settings)

	if m.encoding == Base64 {
		m.hEncoder = bEncoding
	} else {
		m.hEncoder = qEncoding
	}

	return m
}

// Reset resets the message so it can be reused. The message keeps its previous
// settings so it is in the same state that after a call to NewMessage.
func (m *Message) Reset() {
	for k := range m.header {
		delete(m.header, k)
	}
	m.parts = nil
	m.attachments = nil
	m.embedded = nil
}

func (m *Message) applySettings(settings []MessageSetting) {
	for _, s := range settings {
		s(m)
	}
}

// A MessageSetting can be used as an argument in NewMessage to configure an
// email.
type MessageSetting func(m *Message)

// SetCharset is a message setting to set the charset of the email.
func SetCharset(charset string) MessageSetting {
	return func(m *Message) {
		m.charset = charset
	}
}

// SetEncoding is a message setting to set the encoding of the email.
func SetEncoding(enc Encoding) MessageSetting {
	return func(m *Message) {
		m.encoding = enc
	}
}

// Encoding represents a MIME encoding scheme like quoted-printable or base64.
type Encoding string

const (
	// QuotedPrintable represents the quoted-printable encoding as defined in
	// RFC 2045.
	QuotedPrintable Encoding = "quoted-printable"
	// Base64 represents the base64 encoding as defined in RFC 2045.
	Base64 Encoding = "base64"
	// Unencoded can be used to avoid encoding the body of an email. The headers
	// will still be encoded using quoted-printable encoding.
	Unencoded Encoding = "8bit"
)

// SetHeader sets a value to the given header field.
func (m *Message) SetHeader(field string, value ...string) {
	m.encodeHeader(value)
	m.header[field] = value
}

func (m *Message) encodeHeader(values []string) {
	for i := range values {
		values[i] = m.encodeString(values[i])
	}
}

func (m *Message) encodeString(value string) string {
	return m.hEncoder.Encode(m.charset, value)
}

// SetHeaders sets the message headers.
func (m *Message) SetHeaders(h map[string][]string) {
	for k, v := range h {
		m.SetHeader(k, v...)
	}
}

// SetAddressHeader sets an address to the given header field.
func (m *Message) SetAddressHeader(field, address, name string) {
	m.header[field] = []string{m.FormatAddress(address, name)}
}

// FormatAddress formats an address and a name as a valid RFC 5322 address.
func (m *Message) FormatAddress(address, name string) string {
	if name == "" {
		return address
	}

	enc := m.encodeString(name)
	if enc == name {
		m.buf.WriteByte('"')
		for i := 0; i < len(name); i++ {
			b := name[i]
			if b == '\\' || b == '"' {
				m.buf.WriteByte('\\')
			}
			m.buf.WriteByte(b)
		}
		m.buf.WriteByte('"')
	} else if hasSpecials(name) {
		m.buf.WriteString(bEncoding.Encode(m.charset, name))
	} else {
		m.buf.WriteString(enc)
	}
	m.buf.WriteString(" <")
	m.buf.WriteString(address)
	m.buf.WriteByte('>')

	addr := m.buf.String()
	m.buf.Reset()
	return addr
}

func hasSpecials(text string) bool {
	for i := 0; i < len(text); i++ {
		switch c := text[i]; c {
		case '(', ')', '<', '>', '[', ']', ':', ';', '@', '\\', ',', '.', '"':
			return true
		}
	}

	return false
}

// SetDateHeader sets a date to the given header field.
func (m *Message) SetDateHeader(field string, date time.Time) {
	m.header[field] = []string{m.FormatDate(date)}
}

// FormatDate formats a date as a valid RFC 5322 date.
func (m *Message) FormatDate(date time.Time) string {
	return date.Format(time.RFC1123Z)
}

// GetHeader gets a header field.
func (m *Message) GetHeader(field string) []string {
	return m.header[field]
}

// SetBody sets the body of the message. It replaces any content previously set
// by SetBody, AddAlternative or AddAlternativeWriter.
func (m *Message) SetBody(contentType, body string, settings ...PartSetting) {
	m.parts = []*part{m.newPart(contentType, newCopier(body), settings)}
}

// AddAlternative adds an alternative part to the message.
//
// It is commonly used to send HTML emails that default to the plain text
// version for backward compatibility. AddAlternative appends the new part to
// the end of the message. So the plain text part should be added before the
// HTML part. See http://en.wikipedia.org/wiki/MIME#Alternative
func (m *Message) AddAlternative(contentType, body string, settings ...PartSetting) {
	m.AddAlternativeWriter(contentType, newCopier(body), settings...)
}

func newCopier(s string) func(io.Writer) error {
	return func(w io.Writer) error {
		_, err := io.WriteString(w, s)
		return err
	}
}

// AddAlternativeWriter adds an alternative part to the message. It can be
// useful with the text/template or html/template packages.
func (m *Message) AddAlternativeWriter(contentType string, f func(io.Writer) error, settings ...PartSetting) {
	m.parts = append(m.parts, m.newPart(contentType, f, settings))
}

func (m *Message) newPart(contentType string, f func(io.Writer) error, settings []PartSetting) *part {
	p := &part{
		contentType: contentType,
		copier:      f,
		encoding:    m.encoding,
	}

	for _, s := range settings {
		s(p)
	}

	return p
}

// A PartSetting can be used as an argument in Message.SetBody,
// Message.AddAlternative or Message.AddAlternativeWriter to configure the part
// added to a message.
type PartSetting func(*part)

// SetPartEncoding sets the encoding of the part added to the message. By
// default, parts use the same encoding than the message.
func SetPartEncoding(e Encoding) PartSetting {
	return PartSetting(func(p *part) {
		p.encoding = e
	})
}

type file struct {
	Name     string
	Header   map[string][]string
	CopyFunc func(w io.Writer) error
}

func (f *file) setHeader(field, value string) {
	f.Header[field] = []string{value}
}

// A FileSetting can be used as an argument in Message.Attach or Message.Embed.
type FileSetting func(*file)

// SetHeader is a file setting to set the MIME header of the message part that
// contains the file content.
//
// Mandatory headers are automatically added if they are not set when sending
// the email.
func SetHeader(h map[string][]string) FileSetting {
	return func(f *file) {
		for k, v := range h {
			f.Header[k] = v
		}
	}
}

// Rename is a file setting to set the name of the attachment if the name is
// different than the filename on disk.
func Rename(name string) FileSetting {
	return func(f *file) {
		f.Name = name
	}
}

// SetCopyFunc is a file setting to replace the function that runs when the
// message is sent. It should copy the content of the file to the io.Writer.
//
// The default copy function opens the file with the given filename, and copy
// its content to the io.Writer.
func SetCopyFunc(f func(io.Writer) error) FileSetting {
	return func(fi *file) {
		fi.CopyFunc = f
	}
}

func (m *Message) appendFile(list []*file, name string, settings []FileSetting) []*file {
	f := &file{
		Name:   filepath.Base(name),
		Header: make(map[string][]string),
		CopyFunc: func(w io.Writer) error {
			h, err := os.Open(name)
			if err != nil {
				return err
			}
			if _, err := io.Copy(w, h); err != nil {
				h.Close()
				return err
			}
			return h.Close()
		},
	}

	for _, s := range settings {
		s(f)
	}

	if list == nil {
		return []*file{f}
	}

	return append(list, f)
}

// Attach attaches the files to the email.
func (m *Message) Attach(filename string, settings ...FileSetting) {
	m.attachments = m.appendFile(m.attachments, filename, settings)
}

// Embed embeds the images to the email.
func (m *Message) Embed(filename string, settings ...FileSetting) {
	m.embedded = m.appendFile(m.embedded, filename, settings)
}

// WriteTo implements io.WriterTo. It dumps the whole message into w.
func (m *Message) WriteTo(w io.Writer) (int64, error) {
	mw := &messageWriter{w: w}
	mw.writeMessage(m)
	return mw.n, mw.err
}

func (w *messageWriter) writeMessage(m *Message) {
	if _, ok := m.header["Mime-Version"]; !ok {
		w.writeString("Mime-Version: 1.0\r\n")
	}
	if _, ok := m.header["Date"]; !ok {
		w.writeHeader("Date", m.FormatDate(time.Now()))
	}
	w.writeHeaders(m.header)

	if m.hasMixedPart() {
		w.openMultipart("mixed")
	}

	if m.hasRelatedPart() {
		w.openMultipart("related")
	}

	if m.hasAlternativePart() {
		w.openMultipart("alternative")
	}
	for _, part := range m.parts {
		w.writePart(part, m.charset)
	}
	if m.hasAlternativePart() {
		w.closeMultipart()
	}

	w.addFiles(m.embedded, false)
	if m.hasRelatedPart() {
		w.closeMultipart()
	}

	w.addFiles(m.attachments, true)
	if m.hasMixedPart() {
		w.closeMultipart()
	}
}

func (m *Message) hasMixedPart() bool {
	return (len(m.parts) > 0 && len(m.attachments) > 0) || len(m.attachments) > 1
}

func (m *Message) hasRelatedPart() bool {
	return (len(m.parts) > 0 && len(m.embedded) > 0) || len(m.embedded) > 1
}

func (m *Message) hasAlternativePart() bool {
	return len(m.parts) > 1
}

type messageWriter struct {
	w          io.Writer
	n          int64
	writers    [3]*multipart.Writer
	partWriter io.Writer
	depth      uint8
	err        error
}

func (w *messageWriter) openMultipart(mimeType string) {
	mw := multipart.NewWriter(w)
	contentType := "multipart/" + mimeType + ";\r\n boundary=" + mw.Boundary()
	w.writers[w.depth] = mw

	if w.depth == 0 {
		w.writeHeader("Content-Type", contentType)
		w.writeString("\r\n")
	} else {
		w.createPart(map[string][]string{
			"Content-Type": {contentType},
		})
	}
	w.depth++
}

func (w *messageWriter) createPart(h map[string][]string) {
	w.partWriter, w.err = w.writers[w.depth-1].CreatePart(h)
}

func (w *messageWriter) closeMultipart() {
	if w.depth > 0 {
		w.writers[w.depth-1].Close()
		w.depth--
	}
}

func (w *messageWriter) writePart(p *part, charset string) {
	w.writeHeaders(map[string][]string{
		"Content-Type":              {p.contentType + "; charset=" + charset},
		"Content-Transfer-Encoding": {string(p.encoding)},
	})
	w.writeBody(p.copier, p.encoding)
}

func (w *messageWriter) addFiles(files []*file, isAttachment bool) {
	for _, f := range files {
		if _, ok := f.Header["Content-Type"]; !ok {
			mediaType := mime.TypeByExtension(filepath.Ext(f.Name))
			if mediaType == "" {
				mediaType = "application/octet-stream"
			}
			f.setHeader("Content-Type", mediaType+`; name="`+f.Name+`"`)
		}

		if _, ok := f.Header["Content-Transfer-Encoding"]; !ok {
			f.setHeader("Content-Transfer-Encoding", string(Base64))
		}

		if _, ok := f.Header["Content-Disposition"]; !ok {
			var disp string
			if isAttachment {
				disp = "attachment"
			} else {
				disp = "inline"
			}
			f.setHeader("Content-Disposition", disp+`; filename="`+f.Name+`"`)
		}

		if !isAttachment {
			if _, ok := f.Header["Content-ID"]; !ok {
				f.setHeader("Content-ID", "<"+f.Name+">")
			}
		}
		w.writeHeaders(f.Header)
		w.writeBody(f.CopyFunc, Base64)
	}
}

func (w *messageWriter) Write(p []byte) (int, error) {
	if w.err != nil {
		return 0, w.err
	}

	var n int
	n, w.err = w.w.Write(p)
	w.n += int64(n)
	return n, w.err
}

func (w *messageWriter) writeString(s string) {
	n, _ := io.WriteString(w.w, s)
	w.n += int64(n)
}

func (w *messageWriter) writeHeader(k string, v ...string) {
	w.writeString(k)
	if len(v) == 0 {
		w.writeString(":\r\n")
		return
	}
	w.writeString(": ")

	// Max header line length is 78 characters in RFC 5322 and 76 characters
	// in RFC 2047. So for the sake of simplicity we use the 76 characters
	// limit.
	charsLeft := 76 - len(k) - len(": ")

	for i, s := range v {
		// If the line is already too long, insert a newline right away.
		if charsLeft < 1 {
			if i == 0 {
				w.writeString("\r\n ")
			} else {
				w.writeString(",\r\n ")
			}
			charsLeft = 75
		} else if i != 0 {
			w.writeString(", ")
			charsLeft -= 2
		}

		// While the header content is too long, fold it by inserting a newline.
		for len(s) > charsLeft {
			s = w.writeLine(s, charsLeft)
			charsLeft = 75
		}
		w.writeString(s)
		if i := lastIndexByte(s, '\n'); i != -1 {
			charsLeft = 75 - (len(s) - i - 1)
		} else {
			charsLeft -= len(s)
		}
	}
	w.writeString("\r\n")
}

func (w *messageWriter) writeLine(s string, charsLeft int) string {
	// If there is already a newline before the limit. Write the line.
	if i := strings.IndexByte(s, '\n'); i != -1 && i < charsLeft {
		w.writeString(s[:i+1])
		return s[i+1:]
	}

	for i := charsLeft - 1; i >= 0; i-- {
		if s[i] == ' ' {
			w.writeString(s[:i])
			w.writeString("\r\n ")
			return s[i+1:]
		}
	}

	// We could not insert a newline cleanly so look for a space or a newline
	// even if it is after the limit.
	for i := 75; i < len(s); i++ {
		if s[i] == ' ' {
			w.writeString(s[:i])
			w.writeString("\r\n ")
			return s[i+1:]
		}
		if s[i] == '\n' {
			w.writeString(s[:i+1])
			return s[i+1:]
		}
	}

	// Too bad, no space or newline in the whole string. Just write everything.
	w.writeString(s)
	return ""
}

func (w *messageWriter) writeHeaders(h map[string][]string) {
	if w.depth == 0 {
		for k, v := range h {
			if k != "Bcc" {
				w.writeHeader(k, v...)
			}
		}
	} else {
		w.createPart(h)
	}
}

func (w *messageWriter) writeBody(f func(io.Writer) error, enc Encoding) {
	var subWriter io.Writer
	if w.depth == 0 {
		w.writeString("\r\n")
		subWriter = w.w
	} else {
		subWriter = w.partWriter
	}

	if enc == Base64 {
		wc := base64.NewEncoder(base64.StdEncoding, newBase64LineWriter(subWriter))
		w.err = f(wc)
		wc.Close()
	} else if enc == Unencoded {
		w.err = f(subWriter)
	} else {
		wc := newQPWriter(subWriter)
		w.err = f(wc)
		wc.Close()
	}
}

// As required by RFC 2045, 6.7. (page 21) for quoted-printable, and
// RFC 2045, 6.8. (page 25) for base64.
const maxLineLen = 76

// base64LineWriter limits text encoded in base64 to 76 characters per line
type base64LineWriter struct {
	w       io.Writer
	lineLen int
}

func newBase64LineWriter(w io.Writer) *base64LineWriter {
	return &base64LineWriter{w: w}
}

func (w *base64LineWriter) Write(p []byte) (int, error) {
	n := 0
	for len(p)+w.lineLen > maxLineLen {
		w.w.Write(p[:maxLineLen-w.lineLen])
		w.w.Write([]byte("\r\n"))
		p = p[maxLineLen-w.lineLen:]
		n += maxLineLen - w.lineLen
		w.lineLen = 0
	}

	w.w.Write(p)
	w.lineLen += len(p)

	return n + len(p), nil
}
