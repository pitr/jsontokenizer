package jsontokenizer

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"runtime"
	"testing"

	"github.com/matryer/is"
)

func TestToken(t *testing.T) {
	tts := []struct {
		in   string
		toks []TokType
		vals []interface{}
	}{
		{"   ", nil, nil},
		{"null", []TokType{TokNull}, nil},
		{" null", []TokType{TokNull}, nil},
		{"null    null", []TokType{TokNull, TokNull}, nil},
		{"null        null null", []TokType{TokNull, TokNull, TokNull}, nil},

		{"true", []TokType{TokTrue}, nil},
		{" false", []TokType{TokFalse}, nil},
		{"true    true", []TokType{TokTrue, TokTrue}, nil},
		{"false        false false", []TokType{TokFalse, TokFalse, TokFalse}, nil},

		{"-122", []TokType{TokNumber}, []interface{}{[]byte("-122")}},
		{"[-122]", []TokType{TokArrayOpen, TokNumber, TokArrayClose}, []interface{}{[]byte("-122")}},
		{" -122  1111111111E+4", []TokType{TokNumber, TokNumber}, []interface{}{[]byte("-122"), []byte("1111111111E+4")}},
		{`[1,"2"]`, []TokType{TokArrayOpen, TokNumber, TokComma, TokString, TokArrayClose}, []interface{}{[]byte("1"), []byte("2")}},

		{` "hi"`, []TokType{TokString}, []interface{}{[]byte(`hi`)}},
		{`"a loooooooooong" : "string"`, []TokType{TokString, TokObjectColon, TokString}, []interface{}{[]byte(`a loooooooooong`), []byte(`string`)}},
		{`{"k1": "val","k2":42}`, []TokType{TokObjectOpen, TokString, TokObjectColon, TokString, TokComma, TokString, TokObjectColon, TokNumber, TokObjectClose}, []interface{}{[]byte(`k1`), []byte(`val`), []byte(`k2`), []byte(`42`)}},
		{`"1\""`, []TokType{TokString}, []interface{}{[]byte(`1\"`)}},
		{`"1\"4"`, []TokType{TokString}, []interface{}{[]byte(`1\"4`)}},

		{"{", []TokType{TokObjectOpen}, nil},
		{" }", []TokType{TokObjectClose}, nil},
		{"{ {", []TokType{TokObjectOpen, TokObjectOpen}, nil},
		{"{ }", []TokType{TokObjectOpen, TokObjectClose}, nil},

		{"[", []TokType{TokArrayOpen}, nil},
		{" ]", []TokType{TokArrayClose}, nil},
		{"[ [", []TokType{TokArrayOpen, TokArrayOpen}, nil},
		{"[ ]", []TokType{TokArrayOpen, TokArrayClose}, nil},

		{": :", []TokType{TokObjectColon, TokObjectColon}, nil},
		{":::", []TokType{TokObjectColon, TokObjectColon, TokObjectColon}, nil},

		{", ,", []TokType{TokComma, TokComma}, nil},
		{",,,", []TokType{TokComma, TokComma, TokComma}, nil},

		{
			`{"key":"val"}`,
			[]TokType{TokObjectOpen, TokString, TokObjectColon, TokString, TokObjectClose},
			[]interface{}{[]byte(`key`), []byte(`val`)},
		},
	}
	for _, tt := range tts {
		t.Run(tt.in, func(t *testing.T) {
			var (
				is   = is.New(t)
				toks []TokType
				vals []interface{}
				tk   = NewWithSize(bytes.NewBufferString(tt.in), 7)
			)
			for i := 0; i < len(tt.toks); i++ {
				tok, err := tk.Token()
				is.NoErr(err)
				toks = append(toks, tok)
				switch tok {
				case TokNumber:
					var buf bytes.Buffer
					_, err := tk.ReadNumber(&buf)
					is.NoErr(err)
					vals = append(vals, buf.Bytes())
				case TokString:
					var buf bytes.Buffer
					_, err := tk.ReadString(&buf)
					is.NoErr(err)
					vals = append(vals, buf.Bytes())
				case TokNull:
				case TokTrue, TokFalse:
				case TokArrayOpen, TokArrayClose, TokObjectOpen, TokObjectClose:
				}
			}
			is.Equal(toks, tt.toks)
			is.Equal(vals, tt.vals)
			_, err := tk.Token()
			is.Equal(err, io.EOF)
		})
	}
}

func TestToken_Bad(t *testing.T) {
	tts := []struct {
		in  string
		err string
	}{
		{"nil", "expected null got i at index 1"},
		{"hi", `invalid json "hi"`},
		{" fall", "expected false got l at index 3"},
		{" f", "expected false got EOF"},
	}
	for _, tt := range tts {
		t.Run(tt.in, func(t *testing.T) {
			var (
				is = is.New(t)
				tk = New(bytes.NewBufferString(tt.in))
			)
			_, err := tk.Token()
			is.True(err != nil)
			is.Equal(err.Error(), tt.err)
		})
	}
}

func TestReadNumber_Limited(t *testing.T) {
	var (
		is  = is.New(t)
		tk  = NewWithSize(bytes.NewBufferString(`1234567`), 7)
		out = bufio.NewWriterSize(&bytes.Buffer{}, 2)
	)
	tok, err := tk.Token()
	is.NoErr(err)
	is.Equal(tok, TokNumber)
	n, err := tk.ReadNumber(out)
	is.NoErr(err)
	is.Equal(n, 7)

	_, err = tk.Token()
	is.Equal(err, io.EOF)
}

func BenchmarkTokenizer(b *testing.B) {
	var (
		f, _   = os.Open("testdata/big.json")
		buf    = new(bytes.Buffer)
		_, err = io.Copy(buf, f)
		data   = bytes.NewReader(buf.Bytes())
	)

	buf = nil
	if err != nil {
		b.Fatal(err)
	}
	runtime.GC()
	b.ResetTimer()

	for i := 0; i <= 10; i++ {
		size := 8 << i
		b.Run(fmt.Sprintf("size=%d", size), func(b *testing.B) {
			dec := NewWithSize(data, size)

			b.ResetTimer()
			b.ReportAllocs()

			for i := 0; i < b.N; i++ {
				_, _ = data.Seek(0, io.SeekStart)
				dec.Reset(data)

				for {
					tok, err := dec.Token()
					if err != nil {
						if err == io.EOF {
							break
						}
						b.Fatal(err)
					}
					switch tok {
					case TokNumber:
						_, err = dec.ReadNumber(io.Discard)
						if err != nil {
							b.Fatal(err)
						}
					case TokString:
						_, err := dec.ReadString(io.Discard)
						if err != nil {
							b.Fatal(err)
						}
						// do nothing for others
					}
				}
			}
		})
	}
}

func BenchmarkBuiltinDecoder(b *testing.B) {
	var (
		f, _   = os.Open("testdata/big.json")
		buf    = new(bytes.Buffer)
		_, err = io.Copy(buf, f)
		data   = bytes.NewReader(buf.Bytes())
	)

	buf = nil
	if err != nil {
		b.Fatal(err)
	}
	runtime.GC()
	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		_, _ = data.Seek(0, io.SeekStart)
		dec := json.NewDecoder(data)
		dec.UseNumber()

		for {
			tok, err := dec.Token()
			if err != nil {
				if err == io.EOF {
					break
				}
				b.Fatal(err)
			}
			switch tok.(type) {
			case json.Delim:
			case bool:
			case json.Number:
			case string:
			case nil:
			}
		}
	}
}
