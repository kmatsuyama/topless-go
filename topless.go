package main

import (
	"errors"
	"flag"
	"fmt"
	"golang.org/x/sys/unix"
	"log"
	"os"
	"os/exec"
	"strings"
	"time"
	"unicode/utf8"
)

const (
	Before = 0
	After  = 1
	All    = 2
)

const (
	Up      = 'A'
	Down    = 'B'
	Right   = 'C'
	Left    = 'D'
	Below   = 'E'
	Above   = 'F'
	Begin   = 'G'
	Move    = 'H'
	Clear   = 'J'
	Delete  = 'K'
	Forward = 'S'
	Back    = 'T'
)

type strArray struct {
	elem []string
	len  int
}

func newStrArray(str string, delim string) strArray {
	elem := strings.Split(str, delim)
	len := len(elem)
	return strArray{
		elem: elem,
		len:  len,
	}
}

func csiCode(ctrl rune, num ...int) string {
	const CSI = "\033["

	switch len(num) {
	case 1:
		return fmt.Sprintf("%s%d%c", CSI, num[0], ctrl)
	case 2:
		return fmt.Sprintf("%s%d;%d%c", CSI, num[0], num[1], ctrl)
	}
	return ""
}

func getByteLength(b byte) int {
	if b < 0x80 {
		return 1
	} else if 0xc2 <= b && b < 0xe0 {
		return 2
	} else if 0xe0 <= b && b < 0xf0 {
		return 3
	} else if 0xf0 <= b && b < 0xf8 {
		return 4
	} else if 0xf8 <= b && b < 0xfc {
		return 5
	} else if 0xfc <= b && b < 0xfd {
		return 6
	}
	return 0
}

func getStdin(stdinChan chan<- rune) {
	var stdin []byte
	var length int

	input := make([]byte, 1)
	for {
		os.Stdin.Read(input)
		stdin = append(stdin, input[0])
		if length == 0 {
			length = getByteLength(input[0])
		}
		length--
		if length == 0 {
			stdinR, _ := utf8.DecodeRune(stdin)
			stdinChan <- stdinR
			stdin = nil
		} else if length < 0 {
			length = 0
		}
	}
	close(stdinChan)
}

func treatStdin(stdinChan <-chan rune, waitChan chan<- bool) {
	var wait bool
	for stdin := range stdinChan {
		switch stdin {
		case 'q':
			os.Exit(0)
		case 'w':
			wait = !wait
			waitChan <- wait
		}
	}
}

func runCmdstr(cmdstr ...string) (string, error) {
	var cmd *exec.Cmd

	cmdlen := len(cmdstr)
	if cmdlen == 0 {
		return "", errors.New("Command not Found.")
	}

	switch len(cmdstr) {
	case 1:
		cmd = exec.Command(cmdstr[0])
	default:
		cmd = exec.Command(cmdstr[0], cmdstr[1:]...)
	}
	out, err := cmd.CombinedOutput()
	return string(out), err
}

func runCriticalCmd(cmdstr ...string) string {
	out, err := runCmdstr(cmdstr[0:]...)
	if err != nil {
		if out != "" {
			log.Print(out)
		}
		log.Fatal(err)
	}
	return out
}

func runCmdRepeatedly(cmdstr []string, cmdoutChan chan<- string, waitChan <-chan bool, sleepSec uint, force bool) {
	var wait bool

	sleepTime := time.Duration(sleepSec) * time.Second
	for {
		select {
		case wait = <-waitChan:
		default:
		}
		if wait {
			time.Sleep(sleepTime)
			continue
		}
		cmdout, err := runCmdstr(cmdstr[0:]...)
		if !force && err != nil {
			if cmdout != "" {
				log.Print(cmdout)
			}
			log.Fatal(err)
		}
		cmdoutChan <- cmdout
		time.Sleep(sleepTime)
	}
	close(cmdoutChan)
}

func cutExtraLines(oldlinenum int, newlinenum int, height int) {
	if oldlinenum > height {
		oldlinenum = height
	}
	if newlinenum > height {
		newlinenum = height
	}
	if oldlinenum > newlinenum {
		for i := 0; i < oldlinenum-newlinenum; i++ {
			fmt.Print(csiCode(Delete, All))
			fmt.Print(csiCode(Above, 1))
		}
	}
}

func moveToBegin(oldlinenum int, newlinenum int, height int) {
	linenum := oldlinenum

	if oldlinenum > newlinenum {
		linenum = newlinenum
	}
	if linenum > height {
		linenum = height
	}

	if linenum == 0 {
		return
	} else if linenum == 1 {
		fmt.Print(csiCode(Begin, 1))
	} else {
		fmt.Print(csiCode(Above, linenum-1))
	}
}

func printLineDiff(old strArray, new strArray, height int) {
	linenum := new.len

	if linenum > height {
		linenum = height
	}
	for i := 0; i < linenum; i++ {
		if i < old.len && new.elem[i] != "" && old.elem[i] == new.elem[i] {
			fmt.Print(csiCode(Below, 1))
			continue
		}
		fmt.Print(csiCode(Delete, All))
		if i < linenum-1 {
			fmt.Println(new.elem[i])
		} else {
			fmt.Print(new.elem[i])
		}
	}
}

func rewriteLineDiff(old strArray, new strArray, height int) {
	cutExtraLines(old.len, new.len, height)
	moveToBegin(old.len, new.len, height)
	printLineDiff(old, new, height)
}

func getWinHeight() int {
	size, err := unix.IoctlGetWinsize(int(os.Stdout.Fd()), unix.TIOCGWINSZ)
	if err != nil {
		log.Fatal(err)
	}
	return int(size.Row)
}

func rewriteLines(cmdoutChan <-chan string) {
	var oldline strArray
	var newline strArray
	var cmdout string
	var height int

	for {
		height = getWinHeight() - 1
		select {
		case cmdout = <-cmdoutChan:
			newline = newStrArray(cmdout, "\n")
			rewriteLineDiff(oldline, newline, height)
			oldline = newline
		}
	}
}

func main() {
	var sleepSec uint
	var interactive bool
	var shell bool
	var force bool

	flag.Usage = func() {
		fmt.Printf("Usage: %s [-s sec] [-i] [-sh] [-f] command\n\n", os.Args[0])
		flag.PrintDefaults()
	}
	flag.UintVar(&sleepSec, "s", 1, "sleep second")
	flag.BoolVar(&interactive, "i", false, "interactive")
	flag.BoolVar(&shell, "sh", false, "execute through the shell")
	flag.BoolVar(&force, "f", false, "ignore execute error")
	flag.Parse()

	if len(flag.Args()) == 0 {
		flag.Usage()
		os.Exit(1)
	}

	cmd := flag.Args()
	if shell {
		cmd = append([]string{"sh", "-c"}, strings.Join(cmd, " "))
	}

	waitChan := make(chan bool)

	if !interactive {
		runCriticalCmd("stty", "-F", "/dev/tty", "cbreak", "min", "1")
		runCriticalCmd("stty", "-F", "/dev/tty", "-echo")
		defer runCriticalCmd("stty", "-F", "/dev/tty", "echo")
		stdinChan := make(chan rune)
		go treatStdin(stdinChan, waitChan)
		go getStdin(stdinChan)
	}

	cmdoutChan := make(chan string)
	go rewriteLines(cmdoutChan)
	runCmdRepeatedly(cmd, cmdoutChan, waitChan, sleepSec, force)
}
