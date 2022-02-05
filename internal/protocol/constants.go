package protocol

var (
	ErrSyntaxError = NNTPResponse{Code: 501, Message: "Syntax Error"}
)

func IsMessageHeaderAllowed(headerName string) bool {
	switch headerName {
	case
		"Date",
		"From",
		"Message-ID",
		"Newsgroups",
		"Path",
		"Subject",
		"Comments",
		"Keywords",
		"In-Reply-To",
		"Sender",
		"MIME-Version",
		"Content-Type",
		"Content-Transfer-Encoding",
		"Content-Disposition",
		"Content-Language",
		"Approved",
		"Archive",
		"Control",
		"Distribution",
		"Expires",
		"Followup-To",
		"Injection-Date",
		"Injection-Info",
		"Organization",
		"References",
		"Summary",
		"Supersedes",
		"User-Agent",
		"Xref":
		return true
	}
	return false
}

const (
	CRLF            = "\r\n"
	MultilineEnding = "."
)

const (
	CommandCapabilities = "CAPABILITIES"
	CommandQuit         = "QUIT"
	CommandDate         = "DATE"
	CommandMode         = "MODE"
	CommandList         = "LIST"
	CommandGroup        = "GROUP"
	CommandNewGroups    = "NEWGROUPS"
	CommandPost         = "POST"
	CommandListGroup    = "LISTGROUP"
	CommandArticle      = "ARTICLE"
	CommandHead         = "HEAD"
)

const (
	CapabilityNameVersion        = "VERSION"
	CapabilityNameReader         = "READER"
	CapabilityNameIHave          = "IHAVE"
	CapabilityNamePost           = "POST"
	CapabilityNameNewNews        = "NEWNEWS"
	CapabilityNameHdr            = "HDR"
	CapabilityNameOver           = "OVER"
	CapabilityNameList           = "LIST"
	CapabilityNameImplementation = "IMPLEMENTATION"
	CapabilityNameModeReader     = "MODE-READER"
)
