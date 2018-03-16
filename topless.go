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

func getStdin(stdin chan<- string) {
	input := make([]byte, 1)
	for {
		os.Stdin.Read(input)
		stdin <- string(input)
	}
	close(stdin)
}

func treatStdin(stdin <-chan string) {
	for input := range stdin {
		switch input {
		case "q":
			os.Exit(0)
		}
	}
}

func runCmd(cmd *exec.Cmd) (string, error) {
	out, err := cmd.CombinedOutput()
	if err != nil {
		if len(out) != 0 {
			err = errors.New(string(out))
		}
	}
	return string(out), err
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
	cmdout, err := runCmd(cmd)
	return cmdout, err
}

func runCriticalCmd(cmdstr ...string) string {
	out, err := runCmdstr(cmdstr[0:]...)
	if err != nil {
		log.Fatal(err)
	}
	return out
}

func runCmdRepeatedly(cmdstr []string, cmdout chan<- string, sleepSec int) {
	sleepTime := time.Duration(sleepSec) * time.Second
	for {
		output, err := runCmdstr(cmdstr[0:]...)
		if err != nil {
			log.Fatal(err)
		}
		cmdout <- output
		time.Sleep(sleepTime)
	}
	close(cmdout)
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

func printLineDiff(oldlines []string, oldlinenum int, newlines []string, newlinenum int, height int) {
	linenum := newlinenum

	if linenum > height {
		linenum = height
	}
	for i := 0; i < linenum; i++ {
		if i < oldlinenum && newlines[i] != "" && oldlines[i] == newlines[i] {
			fmt.Print(csiCode(Below, 1))
			continue
		}
		fmt.Print(csiCode(Delete, All))
		if i < linenum-1 {
			fmt.Println(newlines[i])
		} else {
			fmt.Print(newlines[i])
		}
	}
}

func getWinHeight() int {
	size, err := unix.IoctlGetWinsize(int(os.Stdout.Fd()), unix.TIOCGWINSZ)
	if err != nil {
		log.Fatal(err)
	}
	return int(size.Row)
}

func rewriteLines(cmdout <-chan string) {
	var oldlines []string
	var oldlinenum int

	for output := range cmdout {
		height := getWinHeight() - 1
		lines := strings.Split(output, "\n")
		linenum := len(lines)
		cutExtraLines(oldlinenum, linenum, height)
		moveToBegin(oldlinenum, linenum, height)
		printLineDiff(oldlines, oldlinenum, lines, linenum, height)
		oldlines = lines
		oldlinenum = linenum
	}
}

func main() {
	var sleepSec int
	var interactive bool
	var shell bool

	flag.Usage = func() {
		fmt.Printf("Usage: %s [-s sec] [-i] [-sh] command\n\n", os.Args[0])
		flag.PrintDefaults()
	}
	flag.IntVar(&sleepSec, "s", 1, "sleep second")
	flag.BoolVar(&interactive, "i", false, "interactive")
	flag.BoolVar(&shell, "sh", false, "execute through the shell")
	flag.Parse()

	if len(flag.Args()) == 0 {
		flag.Usage()
		os.Exit(1)
	}

	cmd := flag.Args()
	if shell {
		cmd = append([]string{"sh", "-c"}, strings.Join(cmd, " "))
	}

	if !interactive {
		runCriticalCmd("stty", "-F", "/dev/tty", "cbreak", "min", "1")
		runCriticalCmd("stty", "-F", "/dev/tty", "-echo")
		defer runCriticalCmd("stty", "-F", "/dev/tty", "echo")
		stdin := make(chan string)
		go treatStdin(stdin)
		go getStdin(stdin)
	}

	cmdout := make(chan string)
	go rewriteLines(cmdout)
	runCmdRepeatedly(cmd, cmdout, sleepSec)
}
