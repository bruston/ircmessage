package ircmessage

import (
	"bufio"
	"bytes"
	"errors"
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

var (
	ErrUnexpectedEOF    = errors.New("unexpected EOF")
	ErrMessageMalformed = errors.New("message malformed")
)

type Scanner struct {
	src            *bufio.Reader
	buf            *bytes.Buffer // Temporary buffer that are re-used where possible.
	rawBuf         []rune        // Keeps track of the current raw IRC message
	currentMsgSize int
	lastRuneSize   int // There is never a need to unread further than one rune, so this is enough.
}

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

type Message struct {
	Raw     string
	Tags    map[string]string
	Prefix  string
	Command string
	Params  []string
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
				return nil, ErrUnexpectedEOF
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
				return "", ErrUnexpectedEOF
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
				return "", ErrUnexpectedEOF
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

func (s *Scanner) readTrailingParam() (string, error) {
	s.buf.Reset()
	for {
		ch, err := s.read()
		if err != nil {
			if err == io.EOF {
				return "", ErrUnexpectedEOF
			}
			return "", err
		}
		if ch == '\r' {
			if ch, _ := s.read(); ch != '\n' {
				return "", ErrMessageMalformed
			}
			break
		}
		s.buf.WriteRune(ch)
	}
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
				return nil, ErrUnexpectedEOF
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

func (s *Scanner) Next() (Message, error) {
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
				return Message{}, ErrUnexpectedEOF
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
