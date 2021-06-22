default: test

test:
	go test .
	golangci-lint run

bench:
	go test -bench=. -run=nothing
