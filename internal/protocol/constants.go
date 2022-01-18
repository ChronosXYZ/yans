package protocol

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

const (
	MessageNNTPServiceReadyPostingProhibited = "201 YANS NNTP Service Ready, posting prohibited"
	MessageReaderModePostingProhibited       = "201 Reader mode, posting prohibited"
	MessageNNTPServiceExitsNormally          = "205 NNTP Service exits normally"
	MessageUnknownCommand                    = "500 Unknown command"
	MessageErrorHappened                     = "403 Failed to process command: "
	MessageListOfNewsgroupsFollows           = "215 list of newsgroups follows"
)
