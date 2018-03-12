package main

import (
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

func runCmd(cmdstr []string, cmdout chan<- string, sleepSec int) {
	cmdlen := len(cmdstr)
	if cmdlen == 0 {
		log.Fatalf("Command not Found.")
	}
	sleepTime := time.Duration(sleepSec) * time.Second
	for {
		var out []byte
		var err error
		switch cmdlen {
		case 1:
			out, err = exec.Command(cmdstr[0]).Output()
		default:
			out, err = exec.Command(cmdstr[0], cmdstr[1:]...).Output()
		}
		if err != nil {
			log.Fatal(err)
		}
		cmdout <- string(out)
		time.Sleep(sleepTime)
	}
}

func printOut(cmdout <-chan string) {
	for {
		out := <-cmdout
		fmt.Print(csiCode(Clear, All))
		fmt.Print(csiCode(Move, 1, 1))
		fmt.Print(out)
	}
}

func moveToBegin(linenum int, height int) {
	if linenum == 0 {
		return
	} else if linenum == 1 {
		fmt.Print(csiCode(Begin, 1))
	} else if linenum > 1 && linenum < height {
		for i := 1; i < linenum; i++ {
			fmt.Print(csiCode(Above, 1))
		}
	} else {
		fmt.Print(csiCode(Move, 1, 1))
	}
}

func printLines(lines []string, linenum int, height int) {
	var num int
	if linenum < height {
		num = linenum
	} else {
		num = height
	}
	for i := 0; i < num; i++ {
		fmt.Print(csiCode(Delete, All))
		if i < num-1 {
			fmt.Println(lines[i])
		} else {
			fmt.Print(lines[i])
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
	oldlinenum := 0
	for {
		out := <-cmdout
		height := getWinHeight()
		moveToBegin(oldlinenum, height)
		lines := strings.Split(out, "\n")
		linenum := len(lines)
		printLines(lines, linenum, height)
		oldlinenum = linenum
	}
}

func main() {
	var sleepSec int
	var interactive bool

	flag.IntVar(&sleepSec, "s", 1, "sleep second")
	flag.BoolVar(&interactive, "i", false, "interactive")

	flag.Parse()

	if len(flag.Args()) == 0 {
		log.Fatalf("Command not Found.")
	}
	if !interactive {
		exec.Command("stty", "-F", "/dev/tty", "cbreak", "min", "1").Run()
		exec.Command("stty", "-F", "/dev/tty", "-echo").Run()
		defer exec.Command("stty", "-F", "/dev/tty", "echo").Run()
	}

	cmdout := make(chan string)
	go runCmd(flag.Args(), cmdout, sleepSec)
	rewriteLines(cmdout)
}
