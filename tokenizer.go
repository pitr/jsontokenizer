package jsontokenizer

import (
	"fmt"
	"io"
)

type TokType int

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

	defaultSize = 64
)

var (
	bnull  = []byte("null")
	btrue  = []byte("true")
	bfalse = []byte("false")

	lookup = [256]byte{0, 0, 0, 0, 0, 0, 0, 0, 0, 's', 's', 0, 0, 's', 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 's', 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, '#', 's', '#', '#', 0, '#', '#', '#', '#', '#', '#', '#', '#', '#', '#', 's', 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, '#', 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, '#'}
)

type Tokenizer struct {
	in   io.Reader
	buf  []byte
	bufp int
	bufe int
}

func New(in io.Reader) *Tokenizer {
	return NewWithSize(in, defaultSize)
}

func NewWithSize(in io.Reader, size int) *Tokenizer {
	return &Tokenizer{in: in, buf: make([]byte, size)}
}

func (t *Tokenizer) Token() (TokType, error) {
	for {
		c, err := t.peek()
		if err != nil {
			return TokNull, err
		}

		switch c {
		case '{':
			t.bufp++
			return TokObjectOpen, nil
		case '}':
			t.bufp++
			return TokObjectClose, nil
		case '[':
			t.bufp++
			return TokArrayOpen, nil
		case ']':
			t.bufp++
			return TokArrayClose, nil
		case '"':
			return TokString, nil
		case 't':
			return TokTrue, t.readWord(btrue)
		case 'f':
			return TokFalse, t.readWord(bfalse)
		case 'n':
			return TokNull, t.readWord(bnull)
		case '-', '0', '1', '2', '3', '4', '5', '6', '7', '8', '9':
			return TokNumber, nil
		default:
			return TokNull, fmt.Errorf("invalid character %q", c)
		}
	}
}

func (t *Tokenizer) ReadNumber(into io.Writer) (n int, err error) {
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

func (t *Tokenizer) ReadString(into io.Writer) (n int, err error) {
	var (
		prev byte
	)

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

func (t *Tokenizer) Reset(in io.Reader) {
	t.bufp = 0
	t.bufe = 0
	t.in = in
}

func (t *Tokenizer) readWord(w []byte) error {
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

func (t *Tokenizer) peek() (byte, error) {
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

func (t *Tokenizer) refill() (err error) {
	t.bufp = 0
	t.bufe, err = t.in.Read(t.buf)

	return err
}
