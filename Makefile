
.PHONY: all
all:
	go build -o bin/gotype cmd/*.go

run:
	./bin/gotype