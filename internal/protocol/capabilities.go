package protocol

import (
	"fmt"
	"strings"
)

type CapabilityType int

const (
	VersionCapability CapabilityType = iota
	ReaderCapability
	IHaveCapability
	PostCapability
	NewNewsCapability
	HdrCapability
	OverCapability
	ListCapability
	ImplementationCapability
	ModeReaderCapability
)

func (ct CapabilityType) String() string {
	switch ct {
	case VersionCapability:
		return CapabilityNameVersion
	case ReaderCapability:
		return CapabilityNameReader
	case IHaveCapability:
		return CapabilityNameIHave
	case PostCapability:
		return CapabilityNamePost
	case NewNewsCapability:
		return CapabilityNameNewNews
	case HdrCapability:
		return CapabilityNameHdr
	case OverCapability:
		return CapabilityNameOver
	case ListCapability:
		return CapabilityNameList
	case ImplementationCapability:
		return CapabilityNameImplementation
	case ModeReaderCapability:
		return CapabilityNameModeReader
	default:
		return ""
	}
}

type Capability struct {
	Type   CapabilityType
	Params string // optional
}

type Capabilities []Capability

func (cs Capabilities) Add(c Capability) {
	for _, v := range cs {
		if v.Type == c.Type {
			return // allowed only unique items
		}
	}
	cs = append(cs, c)
}

func (cs Capabilities) String() string {
	sb := strings.Builder{}
	sb.Write([]byte("101 Capability list:" + CRLF))

	for _, v := range cs {
		if v.Params != "" {
			sb.Write([]byte(fmt.Sprintf("%s %s%s", v.Type, v.Params, CRLF)))
		} else {
			sb.Write([]byte(fmt.Sprintf("%s%s", v.Type, CRLF)))
		}
	}
	sb.Write([]byte(MultilineEnding))

	return sb.String()
}
