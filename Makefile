default: test

test:
	go test .

bench:
	go test -bench=. -run=nothing
