package protocol

const (
	CommandCapabilities = "CAPABILITIES"
	CommandQuit         = "QUIT"
	CommandDate         = "DATE"
	CommandMode         = "MODE"
	CommandList         = "LIST"
)

const (
	MessageNNTPServiceReadyPostingProhibited = "201 YANS NNTP Service Ready, posting prohibited\n"
	MessageReaderModePostingProhibited       = "201 Reader mode, posting prohibited"
	MessageNNTPServiceExitsNormally          = "205 NNTP Service exits normally"
	MessageUnknownCommand                    = "500 Unknown command"
	MessageErrorHappened                     = "403 Failed to process command: "
)
