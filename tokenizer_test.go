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
		vals []string
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

		{"-122", []TokType{TokNumber}, []string{"-122"}},
		{"[-122]", []TokType{TokArrayOpen, TokNumber, TokArrayClose}, []string{"-122"}},
		{" -122  1111111111E+4", []TokType{TokNumber, TokNumber}, []string{"-122", "1111111111E+4"}},
		{`[1,"2"]`, []TokType{TokArrayOpen, TokNumber, TokComma, TokString, TokArrayClose}, []string{"1", "2"}},

		{` "hi"`, []TokType{TokString}, []string{`hi`}},
		{`"a loooooooooong" : "string"`, []TokType{TokString, TokObjectColon, TokString}, []string{`a loooooooooong`, `string`}},
		{`{"k1": "val","k2":42}`, []TokType{TokObjectOpen, TokString, TokObjectColon, TokString, TokComma, TokString, TokObjectColon, TokNumber, TokObjectClose}, []string{`k1`, `val`, `k2`, `42`}},
		{`"1\""`, []TokType{TokString}, []string{`1\"`}},
		{`"1\"4"`, []TokType{TokString}, []string{`1\"4`}},
		{
			`{"m":"\\≢","u":42}`,
			[]TokType{
				TokObjectOpen,
				TokString, TokObjectColon, TokString, TokComma,
				TokString, TokObjectColon, TokNumber,
				TokObjectClose,
			},
			[]string{`m`, `\\≢`, `u`, `42`},
		},

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
			[]string{`key`, `val`},
		},

		// number formats
		{"0", []TokType{TokNumber}, []string{"0"}},
		{"3.14", []TokType{TokNumber}, []string{"3.14"}},
		{"-0.5", []TokType{TokNumber}, []string{"-0.5"}},
		{"1.5e-3", []TokType{TokNumber}, []string{"1.5e-3"}},
		{"1E+10", []TokType{TokNumber}, []string{"1E+10"}},

		// empty string
		{`""`, []TokType{TokString}, []string{""}},

		// mixed literals in array
		{`[null,true,false]`, []TokType{TokArrayOpen, TokNull, TokComma, TokTrue, TokComma, TokFalse, TokArrayClose}, nil},

		// number fills exactly 6 chars after `[`, hitting buffer=7 boundary
		{`[123456]`, []TokType{TokArrayOpen, TokNumber, TokArrayClose}, []string{"123456"}},

		// nested arrays
		{
			`[[1],[2,3]]`,
			[]TokType{TokArrayOpen, TokArrayOpen, TokNumber, TokArrayClose, TokComma, TokArrayOpen, TokNumber, TokComma, TokNumber, TokArrayClose, TokArrayClose},
			[]string{"1", "2", "3"},
		},
	}
	for _, tt := range tts {
		t.Run(tt.in, func(t *testing.T) {
			var (
				is = is.New(t)
				tk = NewWithSize(bytes.NewBufferString(tt.in), 7)
			)
			for i := 0; i < len(tt.toks); i++ {
				tok, err := tk.Token()
				is.NoErr(err)
				is.Equal(tok, tt.toks[i])
				switch tok {
				case TokNumber:
					var buf bytes.Buffer
					_, err := tk.ReadNumber(&buf)
					is.NoErr(err)
					is.Equal(buf.String(), tt.vals[0])
					tt.vals = tt.vals[1:]
				case TokString:
					var buf bytes.Buffer
					_, err := tk.ReadString(&buf)
					is.NoErr(err)
					is.Equal(buf.String(), tt.vals[0])
					tt.vals = tt.vals[1:]
				case TokNull:
				case TokTrue, TokFalse:
				case TokArrayOpen, TokArrayClose, TokObjectOpen, TokObjectClose:
				}
			}
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
		{"tru", "expected true got EOF"},
		{"nul", "expected null got EOF"},
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

// eofWithDataReader returns the final bytes together with io.EOF in a single
// Read. This is legal per the io.Reader contract (os.File and
// net/http/httptest response bodies do it) and must not cause the tokenizer to
// drop the bytes that arrive alongside the EOF.
type eofWithDataReader struct{ data []byte }

func (r *eofWithDataReader) Read(p []byte) (int, error) {
	n := copy(p, r.data)
	r.data = r.data[n:]
	if len(r.data) == 0 {
		return n, io.EOF
	}
	return n, nil
}

func TestToken_EOFWithData(t *testing.T) {
	const in = `{"k1":"a loooooooooong value","k2":1234567}`
	toks := []TokType{
		TokObjectOpen,
		TokString, TokObjectColon, TokString, TokComma,
		TokString, TokObjectColon, TokNumber,
		TokObjectClose,
	}
	want := []string{"k1", "a loooooooooong value", "k2", "1234567"}

	// Exercise a range of buffer sizes so a refill lands mid-token on the
	// final chunk (the case that arrives as (n>0, io.EOF)).
	for _, size := range []int{1, 3, 7, 16, 64} {
		t.Run(fmt.Sprintf("size=%d", size), func(t *testing.T) {
			is := is.New(t)
			tk := NewWithSize(&eofWithDataReader{data: []byte(in)}, size)

			var got []string
			for i := 0; i < len(toks); i++ {
				tok, err := tk.Token()
				is.NoErr(err)
				is.Equal(tok, toks[i])
				switch tok {
				case TokString:
					var buf bytes.Buffer
					_, err := tk.ReadString(&buf)
					is.NoErr(err)
					got = append(got, buf.String())
				case TokNumber:
					var buf bytes.Buffer
					_, err := tk.ReadNumber(&buf)
					is.NoErr(err)
					got = append(got, buf.String())
				}
			}
			_, err := tk.Token()
			is.Equal(err, io.EOF)
			is.Equal(got, want)
		})
	}
}

func TestReset(t *testing.T) {
	is := is.New(t)
	tk := NewWithSize(bytes.NewBufferString("null"), 7)

	tok, err := tk.Token()
	is.NoErr(err)
	is.Equal(tok, TokNull)
	_, err = tk.Token()
	is.Equal(err, io.EOF)

	tk.Reset(bytes.NewBufferString(`"hello"`))
	tok, err = tk.Token()
	is.NoErr(err)
	is.Equal(tok, TokString)
	var buf bytes.Buffer
	_, err = tk.ReadString(&buf)
	is.NoErr(err)
	is.Equal(buf.String(), "hello")
	_, err = tk.Token()
	is.Equal(err, io.EOF)
}

func TestBufferSize1(t *testing.T) {
	tts := []struct {
		in   string
		toks []TokType
		vals []string
	}{
		{"null", []TokType{TokNull}, nil},
		{"true", []TokType{TokTrue}, nil},
		{"false", []TokType{TokFalse}, nil},
		{`""`, []TokType{TokString}, []string{""}},
		{`"hi"`, []TokType{TokString}, []string{"hi"}},
		{`"a\"b"`, []TokType{TokString}, []string{`a\"b`}},
		{`"\\"`, []TokType{TokString}, []string{`\\`}},
		{"42", []TokType{TokNumber}, []string{"42"}},
		{`{"k":1}`, []TokType{TokObjectOpen, TokString, TokObjectColon, TokNumber, TokObjectClose}, []string{"k", "1"}},
	}
	for _, tt := range tts {
		t.Run(tt.in, func(t *testing.T) {
			is := is.New(t)
			tk := NewWithSize(bytes.NewBufferString(tt.in), 1)
			vals := tt.vals
			for _, expected := range tt.toks {
				tok, err := tk.Token()
				is.NoErr(err)
				is.Equal(tok, expected)
				var buf bytes.Buffer
				switch tok {
				case TokString:
					_, err = tk.ReadString(&buf)
					is.NoErr(err)
					is.Equal(buf.String(), vals[0])
					vals = vals[1:]
				case TokNumber:
					_, err = tk.ReadNumber(&buf)
					is.NoErr(err)
					is.Equal(buf.String(), vals[0])
					vals = vals[1:]
				}
			}
			_, err := tk.Token()
			is.Equal(err, io.EOF)
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

func loadTestData(b *testing.B) *bytes.Reader {
	b.Helper()
	f, err := os.Open("testdata/big.json")
	if err != nil {
		b.Fatal(err)
	}
	defer f.Close()
	var buf bytes.Buffer
	if _, err = io.Copy(&buf, f); err != nil {
		b.Fatal(err)
	}
	data := bytes.NewReader(buf.Bytes())
	runtime.GC()
	return data
}

func BenchmarkTokenizer(b *testing.B) {
	data := loadTestData(b)
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
	data := loadTestData(b)
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
