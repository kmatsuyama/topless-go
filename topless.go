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
	elem   []string
	len    int
	orgLen int
}

type optToCmd struct {
	sleepSec uint
	force    bool
}

type stdinToCmd struct {
	wait chan bool
	exit chan bool
}

type stdinToWrite struct {
	refresh chan bool
	head    chan int
}

func newStrArray(str string, delim string, height int) strArray {
	elem := strings.Split(str, delim)
	orgLen := len(elem)
	len := orgLen
	if len > height {
		len = height
	}
	return strArray{
		elem:   elem,
		len:    len,
		orgLen: orgLen,
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

func treatStdin(stdinChan <-chan rune, chanCmd stdinToCmd, chanWrite stdinToWrite) {
	var wait bool
	var exit bool
	for stdin := range stdinChan {
		switch stdin {
		case 'q':
			exit = !exit
			chanCmd.exit <- exit
		case 'w':
			wait = !wait
			chanCmd.wait <- wait
		case 'r':
			chanWrite.refresh <- true
		case '\033':
			stdin2 := <-stdinChan
			switch stdin2 {
			case '[':
				stdin3 := <-stdinChan
				switch stdin3 {
				case Up:
					chanWrite.head <- -1
				case Down:
					chanWrite.head <- +1
				}
			}
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

func runCmdRepeatedly(cmdstr []string, cmdoutChan chan<- string, chanCmd stdinToCmd, optCmd optToCmd) error {
	var cmdout string
	var err error
	var wait bool
	var exit bool

	sleepTime := time.Duration(optCmd.sleepSec) * time.Second
	for {
		select {
		case wait = <-chanCmd.wait:
		case exit = <-chanCmd.exit:
		default:
		}
		if exit {
			break
		}
		if wait {
			time.Sleep(sleepTime)
			continue
		}
		cmdout, err = runCmdstr(cmdstr[0:]...)
		if !optCmd.force && err != nil {
			if cmdout != "" {
				fmt.Print(cmdout)
			}
			fmt.Println(err)
			break
		}
		cmdoutChan <- cmdout
		time.Sleep(sleepTime)
	}
	close(cmdoutChan)
	return err
}

func checkHead(line strArray, head int, dhead int, height int) int {
	if line.orgLen < height {
		return 0
	}
	head = head + dhead
	if head < 0 {
		head = 0
	} else if height+head > line.orgLen {
		head = line.orgLen - height
	}
	return head
}

func eraseToBegin(len int) {
	if len == 0 {
		return
	} else if len == 1 {
		fmt.Print(csiCode(Delete, All))
		fmt.Print(csiCode(Begin, 1))
	} else {
		for i := 0; i < len-1; i++ {
			fmt.Print(csiCode(Delete, All))
			fmt.Print(csiCode(Above, 1))
		}
	}
}

func moveToBegin(len int) {
	if len == 0 {
		return
	} else if len == 1 {
		fmt.Print(csiCode(Begin, 1))
	} else {
		fmt.Print(csiCode(Above, len-1))
	}
}

func printLine(line strArray, head int) {
	len := line.len
	for i := head; i < len+head-1; i++ {
		fmt.Print(csiCode(Delete, All))
		fmt.Println(line.elem[i])
	}
	fmt.Print(csiCode(Delete, All))
	fmt.Print(line.elem[len+head-1])
}

func printLineDiff(old strArray, new strArray, head int) {
	len := new.len
	for i := head; i < len+head; i++ {
		if i < old.len && new.elem[i] != "" && old.elem[i] == new.elem[i] {
			fmt.Print(csiCode(Below, 1))
			continue
		}
		fmt.Print(csiCode(Delete, All))
		if i < len+head-1 {
			fmt.Println(new.elem[i])
		} else {
			fmt.Print(new.elem[i])
		}
	}
}

func rewriteLines(cmdoutChan <-chan string, chanWrite stdinToWrite) {
	var oldline strArray
	var newline strArray
	var cmdout string
	var height int
	var head int

	for {
		winSize, err := unix.IoctlGetWinsize(int(os.Stdout.Fd()), unix.TIOCGWINSZ)
		if err != nil {
			log.Fatal(err)
		}
		height = int(winSize.Row) - 1
		select {
		case refresh := <-chanWrite.refresh:
			if refresh {
				eraseToBegin(oldline.len)
				printLine(oldline, head)
			}
		case dhead := <-chanWrite.head:
			newhead := checkHead(oldline, head, dhead, height)
			if newhead != head {
				head = newhead
				eraseToBegin(oldline.len)
				printLine(oldline, head)
			}
		case cmdout = <-cmdoutChan:
			newline = newStrArray(cmdout, "\n", height)
			head = checkHead(newline, head, 0, height)
			if oldline.len != newline.len {
				eraseToBegin(oldline.len)
				printLine(newline, head)
			} else {
				moveToBegin(oldline.len)
				printLineDiff(oldline, newline, head)
			}
			oldline = newline
		}
	}
}

func main() {
	var interactive bool
	var shell bool
	var optCmd optToCmd
	var err error
	var orgLflag uint32
	var ret int

	flag.Usage = func() {
		fmt.Printf("Usage: %s [-s sec] [-i] [-sh] [-f] command\n\n", os.Args[0])
		flag.PrintDefaults()
	}
	flag.UintVar(&optCmd.sleepSec, "s", 1, "sleep second")
	flag.BoolVar(&interactive, "i", false, "interactive")
	flag.BoolVar(&shell, "sh", false, "execute through the shell")
	flag.BoolVar(&optCmd.force, "f", false, "ignore execute error")
	flag.Parse()

	if len(flag.Args()) == 0 {
		flag.Usage()
		os.Exit(1)
	}

	cmd := flag.Args()
	if shell {
		cmd = append([]string{"sh", "-c"}, strings.Join(cmd, " "))
	}

	chanCmd := stdinToCmd{make(chan bool), make(chan bool)}
	chanWrite := stdinToWrite{make(chan bool), make(chan int)}

	term, err := unix.IoctlGetTermios(int(os.Stdout.Fd()), unix.TCGETS)
	if err != nil {
		log.Fatal(err)
	}
	orgLflag = term.Lflag

	if !interactive {
		term.Lflag &= ^(uint32(unix.ECHO) | uint32(unix.ICANON))
		err = unix.IoctlSetTermios(int(os.Stdout.Fd()), unix.TCSETS, term)
		if err != nil {
			log.Fatal(err)
		}
		stdinChan := make(chan rune)
		go treatStdin(stdinChan, chanCmd, chanWrite)
		go getStdin(stdinChan)
	}

	cmdoutChan := make(chan string)
	go rewriteLines(cmdoutChan, chanWrite)
	err = runCmdRepeatedly(cmd, cmdoutChan, chanCmd, optCmd)
	if err != nil {
		ret = 1
	}
	term.Lflag = orgLflag
	err = unix.IoctlSetTermios(int(os.Stdout.Fd()), unix.TCSETS, term)
	if err != nil {
		log.Fatal(err)
	}
	os.Exit(ret)
}
