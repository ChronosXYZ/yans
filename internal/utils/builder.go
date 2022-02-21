package utils

import (
	"github.com/jhillyerd/enmime"
	"io/ioutil"
	"mime"
	"net/textproto"
	"os"
	"path/filepath"
	"reflect"
)

const (
	cdAttachment = "attachment"
	cdInline     = "inline"

	ctMultipartAltern  = "multipart/alternative"
	ctMultipartMixed   = "multipart/mixed"
	ctMultipartRelated = "multipart/related"
	ctTextPlain        = "text/plain"
	ctTextHTML         = "text/html"

	hnMIMEVersion = "MIME-Version"

	utf8 = "utf-8"
)

// MailBuilder facilitates the easy construction of a MIME message.  Each manipulation method
// returns a copy of the receiver struct.  It can be considered immutable if the caller does not
// modify the string and byte slices passed in.  Immutability allows the headers or entire message
// to be reused across multiple threads.
type MailBuilder struct {
	header               textproto.MIMEHeader
	text, html           []byte
	inlines, attachments []*enmime.Part
	err                  error
}

// Builder returns an empty MailBuilder struct.
func Builder() MailBuilder {
	return MailBuilder{}
}

// Error returns the stored error from a file attachment/inline read or nil.
func (p MailBuilder) Error() error {
	return p.err
}

// Header returns a copy of MailBuilder with the specified value added to the named header.
func (p MailBuilder) Header(name, value string) MailBuilder {
	// Copy existing header map
	h := textproto.MIMEHeader{}
	for k, v := range p.header {
		h[k] = v
	}
	h.Add(name, value)
	p.header = h
	return p
}

// Text returns a copy of MailBuilder that will use the provided bytes for its text/plain Part.
func (p MailBuilder) Text(body []byte) MailBuilder {
	p.text = body
	return p
}

// HTML returns a copy of MailBuilder that will use the provided bytes for its text/html Part.
func (p MailBuilder) HTML(body []byte) MailBuilder {
	p.html = body
	return p
}

// AddAttachment returns a copy of MailBuilder that includes the specified attachment.
func (p MailBuilder) AddAttachment(b []byte, contentType string, fileName string) MailBuilder {
	part := enmime.NewPart(contentType)
	part.Content = b
	part.FileName = fileName
	part.Disposition = cdAttachment
	p.attachments = append(p.attachments, part)
	return p
}

// AddFileAttachment returns a copy of MailBuilder that includes the specified attachment.
// fileName, will be populated from the base name of path.  Content type will be detected from the
// path extension.
func (p MailBuilder) AddFileAttachment(path string) MailBuilder {
	// Only allow first p.err value
	if p.err != nil {
		return p
	}
	f, err := os.Open(path)
	if err != nil {
		p.err = err
		return p
	}
	b, err := ioutil.ReadAll(f)
	if err != nil {
		p.err = err
		return p
	}
	name := filepath.Base(path)
	ctype := mime.TypeByExtension(filepath.Ext(name))
	return p.AddAttachment(b, ctype, name)
}

// AddInline returns a copy of MailBuilder that includes the specified inline.  fileName and
// contentID may be left empty.
func (p MailBuilder) AddInline(
	b []byte,
	contentType string,
	fileName string,
	contentID string,
) MailBuilder {
	part := enmime.NewPart(contentType)
	part.Content = b
	part.FileName = fileName
	part.Disposition = cdInline
	part.ContentID = contentID
	p.inlines = append(p.inlines, part)
	return p
}

// AddFileInline returns a copy of MailBuilder that includes the specified inline.  fileName and
// contentID will be populated from the base name of path.  Content type will be detected from the
// path extension.
func (p MailBuilder) AddFileInline(path string) MailBuilder {
	// Only allow first p.err value
	if p.err != nil {
		return p
	}
	f, err := os.Open(path)
	if err != nil {
		p.err = err
		return p
	}
	b, err := ioutil.ReadAll(f)
	if err != nil {
		p.err = err
		return p
	}
	name := filepath.Base(path)
	ctype := mime.TypeByExtension(filepath.Ext(name))
	return p.AddInline(b, ctype, name, name)
}

// Build performs some basic validations, then constructs a tree of Part structs from the configured
// MailBuilder.  It will set the Date header to now if it was not explicitly set.
func (p MailBuilder) Build() (*enmime.Part, error) {
	if p.err != nil {
		return nil, p.err
	}
	// Validations
	// Fully loaded structure; the presence of text, html, inlines, and attachments will determine
	// how much is necessary:
	//
	//  multipart/mixed
	//  |- multipart/related
	//  |  |- multipart/alternative
	//  |  |  |- text/plain
	//  |  |  `- text/html
	//  |  `- inlines..
	//  `- attachments..
	//
	// We build this tree starting at the leaves, re-rooting as needed.
	var root, part *enmime.Part
	if p.text != nil || p.html == nil {
		root = enmime.NewPart(ctTextPlain)
		root.Content = p.text
		root.Charset = utf8
	}
	if p.html != nil {
		part = enmime.NewPart(ctTextHTML)
		part.Content = p.html
		part.Charset = utf8
		if root == nil {
			root = part
		} else {
			root.NextSibling = part
		}
	}
	if p.text != nil && p.html != nil {
		// Wrap Text & HTML bodies
		part = root
		root = enmime.NewPart(ctMultipartAltern)
		root.AddChild(part)
	}
	if len(p.inlines) > 0 {
		part = root
		root = enmime.NewPart(ctMultipartRelated)
		root.AddChild(part)
		for _, ip := range p.inlines {
			// Copy inline Part to isolate mutations
			part = &enmime.Part{}
			*part = *ip
			part.Header = make(textproto.MIMEHeader)
			root.AddChild(part)
		}
	}
	if len(p.attachments) > 0 {
		part = root
		root = enmime.NewPart(ctMultipartMixed)
		root.AddChild(part)
		for _, ap := range p.attachments {
			// Copy attachment Part to isolate mutations
			part = &enmime.Part{}
			*part = *ap
			part.Header = make(textproto.MIMEHeader)
			root.AddChild(part)
		}
	}
	// Headers
	h := root.Header
	h.Set(hnMIMEVersion, "1.0")
	for k, v := range p.header {
		for _, s := range v {
			h.Set(k, s)
		}
	}
	return root, nil
}

// Equals uses the reflect package to test two MailBuilder structs for equality, primarily for unit
// tests.
func (p MailBuilder) Equals(o MailBuilder) bool {
	return reflect.DeepEqual(p, o)
}
