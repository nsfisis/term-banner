BIN := term-banner

all: build

.PHONY: run
run: build
	./$(BIN)

.PHONY: build
build: $(BIN)

$(BIN): main.go
	go build -o $(BIN)

.PHONY: fmt
fmt:
	go fmt

.PHONY: clean
clean:
	go clean

test: build
	./$(BIN) 'Hello, World!' 'こんにちは、' '世界！'
