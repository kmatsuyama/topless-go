
TARGET = topless
DEST   = /usr/local/bin
VPATH  = ioctl:stdout

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

topless: topless.go ioctl_darwin.go ioctl_linux.go stdout.go
	go build -v $<
