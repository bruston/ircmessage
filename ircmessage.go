// Package ircmessage provides a scanner capable of parsing RFC1459-compliant IRC messages,
// with support for IRCv3 message tags.
package ircmessage

import (
	"bufio"
	"bytes"
	"errors"
	"fmt"
	"io"
	"strings"
)

const (
	maxMessageSize = 512
	runeAt         = '@'
	runeColon      = ':'
	runeSemicolon  = ';'
	runeSpace      = ' '
	runeEquals     = '='
	tokenEquals    = "="
	tokenColon     = ":"
	tokenSemicolon = ";"
	tokenSpace     = " "
)

// ErrMessageMalformed is returned when the scanner encounters a malformed message.
// The only other  error ircmessage specifically returns is io.ErrUnexpectedEOF.
// Any other error you encounter comes from the source reader.
var ErrMessageMalformed = errors.New("message malformed")

// Scanner provides a convenient interface for parsing RFC1459-compliant IRC messages,
// with support for IRCv3 message tags.
//
// Scanning stops unrecoverably at EOF, the first I/O error, or a malformed message.
// When a scan stops, the reader may have advanced arbitrarily far past the last message.
type Scanner struct {
	src            *bufio.Reader
	buf            *bytes.Buffer // Temporary buffer that is re-used where possible.
	rawBuf         []rune        // Keeps track of the current raw IRC message.
	message        Message       // Last message parsed.
	err            error         // Last error encountered.
	currentMsgSize int
	lastRuneSize   int // There is never a need to unread further than one rune, so this is enough.
}

// NewScanner returns a new Scanner to read from r.
func NewScanner(r io.Reader) *Scanner {
	return &Scanner{
		src: bufio.NewReader(r),
		buf: &bytes.Buffer{},
	}
}

func (s *Scanner) read() (rune, error) {
	rn, n, err := s.src.ReadRune()
	if err != nil {
		return 0, err
	}
	s.lastRuneSize = n
	s.currentMsgSize += n
	s.rawBuf = append(s.rawBuf, rn)
	if s.currentMsgSize > maxMessageSize {
		return 0, ErrMessageMalformed
	}
	return rn, err
}

func (s *Scanner) unread() error {
	if err := s.src.UnreadRune(); err != nil {
		return err
	}
	s.currentMsgSize -= s.lastRuneSize
	s.rawBuf = s.rawBuf[:len(s.rawBuf)-1]
	return nil
}

// Message represents a parsed IRC message.
type Message struct {
	Raw     string
	Tags    map[string]string
	Prefix  string
	Command string
	Params  []string
}

func (m Message) String() string {
	return fmt.Sprintf("Raw: %s\nTags: %#v\nPrefix: %s\nCommand: %s\nParams: %#v\n",
		m.Raw,
		m.Tags,
		m.Prefix,
		m.Command,
		m.Params,
	)
}

func (s *Scanner) skipSpace() {
	for {
		ch, _ := s.read()
		if ch != runeSpace {
			s.unread()
			break
		}
	}
}

func (s *Scanner) readTags() (map[string]string, error) {
	// Read whole tag string.
	s.buf.Reset()
	for {
		ch, err := s.read()
		if err != nil {
			if err == io.EOF {
				return nil, io.ErrUnexpectedEOF
			}
			return nil, err
		}
		if ch == runeSpace {
			break
		}
		if s.buf.Len() >= maxMessageSize {
			return nil, ErrMessageMalformed
		}
		s.buf.WriteRune(ch)
	}
	// Split tags.
	tagMap := make(map[string]string)
	var tags []string
	if !strings.Contains(s.buf.String(), tokenSemicolon) {
		tags = append(tags, s.buf.String())
	} else {
		splitTags := strings.Split(s.buf.String(), tokenSemicolon)
		for _, v := range splitTags {
			if strings.Contains(v, tokenEquals) {
				pair := strings.Split(v, tokenEquals)
				if len(pair) < 2 || len(pair) > 2 {
					return nil, ErrMessageMalformed
				}
				tagMap[pair[0]] = pair[1]
				continue
			}
			tagMap[v] = ""
		}
	}
	s.skipSpace()
	return tagMap, nil
}

func (s *Scanner) readPrefix() (string, error) {
	s.buf.Reset()
	for {
		ch, err := s.read()
		if err != nil {
			if err == io.EOF {
				return "", io.ErrUnexpectedEOF
			}
			return "", err
		}
		if ch == runeSpace {
			break
		}
		s.buf.WriteRune(ch)
	}
	prefix := s.buf.String()
	s.skipSpace()
	return prefix, nil
}

func (s *Scanner) readCommand() (string, error) {
	s.buf.Reset()
	for {
		ch, err := s.read()
		if err != nil {
			if err == io.EOF {
				return "", io.ErrUnexpectedEOF
			}
			return "", err
		}
		if ch == runeSpace {
			break
		}
		if ch == '\r' {
			s.unread()
			break
		}
		s.buf.WriteRune(ch)
	}
	s.skipSpace()
	return s.buf.String(), nil
}

func (s *Scanner) readParams() ([]string, error) {
	var params []string
	s.buf.Reset()
	for {
		if end, _ := s.isLineEnd(); end {
			break
		}
		ch, err := s.read()
		if err != nil {
			if err == io.EOF {
				return nil, io.ErrUnexpectedEOF
			}
			return nil, err
		}
		s.buf.WriteRune(ch)
	}
	// A colon indicates a trailing parameter, read
	// everything from after the colon to line ending
	// and append it to params.
	paramString := strings.Split(s.buf.String(), tokenSpace)
	for i, v := range paramString {
		if strings.HasPrefix(v, tokenColon) {
			paramString[i] = paramString[i][1:]
			params = append(params, strings.Join(paramString[i:], tokenSpace))
			break
		}
		if v != "" {
			params = append(params, v)
		}
	}
	return params, nil
}

func (s *Scanner) isLineEnd() (bool, error) {
	ch, err := s.read()
	if err != nil {
		return false, err
	}
	if ch == '\r' {
		ch, err := s.read()
		if err != nil {
			return false, err
		}
		if ch == '\n' {
			return true, nil
		}
		s.unread()
	}
	s.unread()
	return false, nil
}

func (s *Scanner) next() (Message, error) {
	s.rawBuf = make([]rune, 0, 1024)
	s.currentMsgSize = 0
	var msg Message
	ch, err := s.read()
	if err != nil {
		return Message{}, err
	}
	// Check for and read message tags if present as per:
	// http://ircv3.net/specs/core/message-tags-3.2.html
	if ch == runeAt {
		msg.Tags, err = s.readTags()
		if err != nil {
			return Message{}, err
		}
		// Reset the size counter. Tags can be a maximum of 512 bytes
		// and the remainder of the message is allowed a further 512.
		s.currentMsgSize = 0
		// Get next rune
		ch, err = s.read()
		if err != nil {
			if err == io.EOF {
				return Message{}, io.ErrUnexpectedEOF
			}
			return Message{}, err
		}
	}
	// Read message prefix if present, prefixes are
	// prepended with a colon.
	if ch == runeColon {
		msg.Prefix, err = s.readPrefix()
		if err != nil {
			return Message{}, err
		}
	} else {
		s.unread()
	}
	msg.Command, err = s.readCommand()
	if err != nil {
		return Message{}, err
	}
	// Check for line ending, else start reading params.
	end, err := s.isLineEnd()
	if err != nil {
		return Message{}, err
	}
	if end {
		msg.Raw = string(s.rawBuf)
		return msg, nil
	}
	s.unread()
	msg.Params, err = s.readParams()
	if err != nil {
		return Message{}, err
	}
	msg.Raw = string(s.rawBuf)
	return msg, nil
}

// Scan advances the Scanner to the next message, which is then available
// through the Message method. It returns false when the scan stops, either
// by reaching the end of the input or an error. After Scan returns false,
// the Err method will return any error that occured during scanning, the
// exception being if it was io.EOF, in which case Err will return nil.
func (s *Scanner) Scan() bool {
	if s.err != nil {
		return false
	}
	msg, err := s.next()
	if err != nil {
		s.err = err
		return false
	}
	s.message = msg
	return true
}

// Message returns the most recent Message generated by a call to Scan.
func (s *Scanner) Message() Message { return s.message }

// Err returns the first non-EOF error that was encountered by the
// Scanner.
func (s *Scanner) Err() error {
	if s.err == nil || s.err == io.EOF {
		return nil
	}
	return s.err
}

// Prefix represents a parsed IRC message prefix.
type Prefix struct {
	Raw string
	// Indicates whether the prefix is a server. If
	// false, the prefix is a user.
	IsServer bool
	Nickname string
	User     string
	Host     string
}

// ParsePrefix accepts a string prefix and returns a
// parsed *Prefix or nil if the input was invalid.
func ParsePrefix(in string) *Prefix {
	if len(in) == 0 {
		return nil
	}
	if in[0] == '!' || in[0] == '@' {
		return nil
	}
	dpos := strings.Index(in, ".") + 1
	upos := strings.Index(in, "!") + 1
	hpos := strings.Index(in[upos:], "@") + 1 + upos
	p := &Prefix{Raw: in}
	if upos > 0 {
		p.Nickname = in[:upos-1]
		if hpos > 0 && upos < hpos {
			p.User = in[upos : hpos-1]
			p.Host = in[hpos:]
		} else {
			p.User = in[upos:]
		}
	} else if hpos > 0 {
		p.Nickname = in[:hpos-1]
		p.Host = in[hpos:]
	} else if dpos > 0 {
		p.Host = in
		p.IsServer = true
	} else {
		p.Nickname = in
	}
	return p
}
