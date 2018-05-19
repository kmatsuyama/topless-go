
TARGET = topless
DEST   = /usr/local/bin

.SUFFIXES: .go

.PHONY: all
all: $(TARGET)

.PHONY: install
install: $(TARGET)
	install -s $(TARGET) $(DEST)

.PHONY: uninstall
uninstall:
	rm -f $(DEST)/$(TARGET)

.PHONY: clean
clean:
	rm -f $(TARGET)

.PHONY: deps
deps:
	go get golang.org/x/sys/unix

.go:
	go build -v $<
