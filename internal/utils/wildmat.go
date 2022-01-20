package utils

import (
	"fmt"
	"github.com/dlclark/regexp2"
	"strings"
)

type Wildmat struct {
	patterns []*WildmatPattern
}

type WildmatPattern struct {
	negated bool
	pattern string
	regex   *regexp2.Regexp
}

func regexpEscape(str string) string {
	restrictedChars := []string{"(\\", "+", "|", "{", "[", "(", ")", "^", "$", ".", "#"}
	for _, v := range restrictedChars {
		str = strings.ReplaceAll(str, v, "\\"+v)
	}
	return str
}

func convertWildmatToRegex(pat string) (*regexp2.Regexp, error) {
	regex := ""
	pat = regexpEscape(pat)
	patRunes := []rune(pat)
	for _, v := range patRunes {
		switch v {
		case '?':
			regex += "."
		case '*':
			regex += ".?"
		default:
			{
				regex += string(v)
			}
		}
	}
	return regexp2.Compile(regex, regexp2.None)
}

func ParseWildmat(wildmat string) (*Wildmat, error) {
	res := &Wildmat{}
	for _, v := range strings.Split(wildmat, ",") {
		if len(v) > 0 && v[0] == '!' {
			r, err := convertWildmatToRegex(v[1:])
			if err != nil {
				return nil, err
			}
			res.patterns = append(res.patterns, &WildmatPattern{pattern: v[1:], negated: true, regex: r})
		} else {
			r, err := convertWildmatToRegex(v)
			if err != nil {
				return nil, err
			}
			res.patterns = append(res.patterns, &WildmatPattern{pattern: v, negated: false, regex: r})
		}
	}
	return res, nil
}

func (w *Wildmat) ToRegex() (*regexp2.Regexp, error) {
	res := "(%s)%s"
	include := ""
	exclude := ""
	for _, v := range w.patterns {
		if v.negated {
			exclude += fmt.Sprintf("(?!%s)", v.regex.String())
		} else {
			if len(include) != 0 {
				include += fmt.Sprintf("|(%s)", v.regex.String())
			} else {
				include += fmt.Sprintf("(%s)", v.regex.String())
			}
		}
	}
	res = fmt.Sprintf(res, include, exclude)
	return regexp2.Compile(res, regexp2.None)
}
