# json tokenizer

Zero-allocation JSON tokenizer.

## Features

- Fast. ~15x faster than `encoding/json.Decoder`. See benchmarks below.
- Similar API to `encoding/json.Decoder`.
- No reflection.
- No allocations, beyond small buffer for reading.
- Can be reused with a call to `Reset`.

## Anti-Features

- Does **NOT** parse JSON. Will not verify semantic correctness. `[}` will produce 2 tokens without errors.
- Needs an `io.Writer` to write numbers and strings into. Based on your use case, can be `os.Stdout`, `bytes.Buffer`, [ByteBuffer](https://github.com/valyala/bytebufferpool), etc.
- Does not escape strings. `"he is 5'11\\"."` will be exactly that.
- Does not parse numbers into floats/ints. Use `strconv.Atoi()` if needed.
- Not thread safe.

## Quick Start

```go
import json "github.com/pitr/jsontokenizer"

func example(in io.Reader) error {
	tk := json.New(in)

	for {
		tok, err := tk.Token()
		if err == io.EOF {
			return nil
		}
		if err != nil {
			return err
		}
		switch tok {
		case json.TokNull:
			println("got null")
		case json.TokTrue, json.TokFalse:
			println("got bool")
		case json.TokArrayOpen, json.TokArrayClose, json.TokObjectOpen, json.TokObjectClose:
			println("got delimiter")
		case json.TokNumber:
			println("got number")
			_, err := tk.ReadNumber(io.Discard)
			if err != nil {
				return err
			}
		case json.TokString:
			println("got string")
			_, err := tk.ReadString(io.Discard)
			if err != nil {
				return err
			}
		}
	}
}
```

## Benchmarks

Sizes are buffer sizes, which can be specified with `NewWithSize`. Default is 64. Tokenizer is re-used between benchmark iterations, but this doesn't impact performance.

`BenchmarkBuiltinDecoder` is `encoding/json.Decoder`.

```
BenchmarkTokenizer/size=8-8         	    1419	    788208 ns/op	       0 B/op	       0 allocs/op
BenchmarkTokenizer/size=16-8         	    1668	    688656 ns/op	       0 B/op	       0 allocs/op
BenchmarkTokenizer/size=32-8         	    1792	    628601 ns/op	       0 B/op	       0 allocs/op
BenchmarkTokenizer/size=64-8         	    2040	    571411 ns/op	       0 B/op	       0 allocs/op
BenchmarkTokenizer/size=128-8        	    2228	    520646 ns/op	       0 B/op	       0 allocs/op
BenchmarkTokenizer/size=256-8        	    2392	    482151 ns/op	       0 B/op	       0 allocs/op
BenchmarkTokenizer/size=512-8        	    2516	    460283 ns/op	       0 B/op	       0 allocs/op
BenchmarkTokenizer/size=1024-8       	    2553	    458148 ns/op	       0 B/op	       0 allocs/op
BenchmarkTokenizer/size=2048-8       	    2618	    451937 ns/op	       0 B/op	       0 allocs/op
BenchmarkTokenizer/size=4096-8       	    2499	    451601 ns/op	       0 B/op	       0 allocs/op
BenchmarkTokenizer/size=8192-8       	    2610	    443493 ns/op	       0 B/op	       0 allocs/op

BenchmarkBuiltinDecoder-8            	     157	   7607729 ns/op	 1755495 B/op	  107836 allocs/op
```
