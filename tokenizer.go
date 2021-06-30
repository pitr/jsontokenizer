package jsontokenizer

import (
	"fmt"
	"io"
)

// A TokType is an enum for JSON types.
type TokType int

// The following TokTypes are defined.
const (
	TokNull TokType = iota
	TokTrue
	TokFalse
	TokNumber
	TokString
	TokArrayOpen
	TokArrayClose
	TokObjectOpen
	TokObjectClose
	TokObjectColon
	TokComma

	defaultSize = 64
)

// Tokenizer reads and tokenizes JSON from an input stream.
type Tokenizer interface {
	// Token returns next token. TokString and TokNumber tokens must be
	// consumed by ReadString and ReadNumber respectively.
	Token() (TokType, error)
	// ReadNumber consumes number token by writing it into provided io.Writer.
	ReadNumber(into io.Writer) (n int, err error)
	// ReadString consumes string token by writing it into provided io.Writer.
	ReadString(into io.Writer) (n int, err error)
	// Reset resets state of Tokenizer so it can be re-used with another Reader.
	Reset(in io.Reader)
}

var (
	_ Tokenizer = &tokenizer{}

	bnull  = []byte("null")
	btrue  = []byte("true")
	bfalse = []byte("false")

	lookup = [256]byte{
		'\t': 's', '\n': 's', '\r': 's', ' ': 's',
		'+': '#', '-': '#', '.': '#', '0': '#', '1': '#',
		'2': '#', '3': '#', '4': '#', '5': '#', '6': '#',
		'7': '#', '8': '#', '9': '#', 'E': '#', 'e': '#',
	}
	toklookup = [256]TokType{
		'{': TokObjectOpen, '}': TokObjectClose,
		'[': TokArrayOpen, ']': TokArrayClose,
		':': TokObjectColon, ',': TokComma,
		'"': TokString, '-': TokNumber, '0': TokNumber,
		'1': TokNumber, '2': TokNumber, '3': TokNumber,
		'4': TokNumber, '5': TokNumber, '6': TokNumber,
		'7': TokNumber, '8': TokNumber, '9': TokNumber,
	}
)

type tokenizer struct {
	in   io.Reader
	buf  []byte
	bufp int
	bufe int
}

// New returns a new Tokenizer with default buffer size.
func New(in io.Reader) Tokenizer {
	return NewWithSize(in, defaultSize)
}

// NewWithSize returns a new Tokenizer with custom buffer size.
func NewWithSize(in io.Reader, size int) Tokenizer {
	return &tokenizer{in: in, buf: make([]byte, size)}
}

func (t *tokenizer) Token() (TokType, error) {
	c, err := t.peek()
	if err != nil {
		return TokNull, err
	}

	switch toklookup[c] {
	case 0:
		switch c {
		case 't':
			return TokTrue, t.readWord(btrue)
		case 'f':
			return TokFalse, t.readWord(bfalse)
		case 'n':
			return TokNull, t.readWord(bnull)
		default:
			return TokNull, fmt.Errorf("invalid json %q", t.buf[t.bufp:t.bufe])
		}
	case TokObjectOpen, TokObjectClose, TokArrayOpen, TokArrayClose, TokObjectColon, TokComma:
		t.bufp++
		fallthrough
	default:
		return toklookup[c], nil
	}
}

func (t *tokenizer) ReadNumber(into io.Writer) (n int, err error) {
	for {
		for i := t.bufp; i < t.bufe; i++ {
			if lookup[t.buf[i]] == '#' {
				continue
			}
			z, err := into.Write(t.buf[t.bufp:i])
			n += z
			if err != nil {
				return n, err
			}
			t.bufp = i
			return n, nil
		}
		z, err := into.Write(t.buf[t.bufp:t.bufe])
		n += z
		if err != nil {
			return n, err
		}
		err = t.refill()
		if err == io.EOF {
			return n, nil
		}
		if err != nil {
			return n, err
		}
	}
}

func (t *tokenizer) ReadString(into io.Writer) (n int, err error) {
	var prev byte

	t.bufp++

	for {
		for i, c := range t.buf[t.bufp:t.bufe] {
			if c == '"' && prev != '\\' {
				z, err := into.Write(t.buf[t.bufp : t.bufp+i])
				n += z
				t.bufp += i + 1
				return n, err
			}
			prev = c
		}
		z, err := into.Write(t.buf[t.bufp:t.bufe])
		n += z
		if err != nil {
			return n, err
		}
		err = t.refill()
		if err == io.EOF {
			return n, nil
		}
		if err != nil {
			return n, err
		}
	}
}

func (t *tokenizer) Reset(in io.Reader) {
	t.bufp = 0
	t.bufe = 0
	t.in = in
}

func (t *tokenizer) readWord(w []byte) error {
	for i, c := range w {
		if t.bufe == t.bufp {
			err := t.refill()
			if err != nil {
				if err == io.EOF {
					return fmt.Errorf("expected %s got EOF", w)
				}
				return err
			}
		}

		if t.buf[t.bufp] != c {
			return fmt.Errorf("expected %s got %c at index %d", w, t.buf[t.bufp], i)
		}
		t.bufp++
	}

	return nil
}

func (t *tokenizer) peek() (byte, error) {
	var err error
	for {
		for i := t.bufp; i < t.bufe; i++ {
			c := t.buf[i]
			if lookup[c] == 's' {
				continue
			}
			t.bufp = i
			return c, nil
		}
		if err != nil {
			return 0, err
		}
		err = t.refill()
	}
}

func (t *tokenizer) refill() (err error) {
	t.bufp = 0
	t.bufe, err = t.in.Read(t.buf)

	return err
}
