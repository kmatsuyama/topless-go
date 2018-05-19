package main

import (
	"bytes"
	"flag"
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
	"strings"
	"time"
	"./ioctl"
)

const (
	StdinBuf = 128
	SleepSecDef = 1
	CountMaxDef = 3
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

const (
	CSI     = "\033["
	CtrlD   = "\004"
	CtrlU   = "\025"
	UpKey   = "\033[A"
	DownKey = "\033[B"
)

const (
	Normal  = "\x1B[0m"
	Red     = "\x1B[31m"
	Green   = "\x1B[32m"
	Yellow  = "\x1B[33m"
	Blue    = "\x1B[34m"
	Magenta = "\x1B[35m"
	Cyan    = "\x1B[36m"
	White   = "\x1B[37m"
)

type strArray struct {
	elem      []string
	colorElem []string
	length    int
	height    int
	width     int
	count     []int
}

type optToCmd struct {
	sleepSec float64
	force    bool
}

type stdinToCmd struct {
	wait chan bool
	exit chan bool
}

type stdinToPrint struct {
	refresh chan bool
	head    chan int
}

type printFn func(...interface{}) (n int, err error)
type printLine func(int, strArray, printFn) ()

func newStrArray(str string, delim string, height int, width int) strArray {
	elem := strings.Split(str, delim)
	length := len(elem)
	if length < height {
		height = length
	}
	return strArray{
		elem:   elem,
		colorElem:  make([]string, length),
		length: length,
		height: height,
		width: width,
		count: make([]int, length),
	}
}

func csiCode(ctrl rune, num ...int) string {
	switch len(num) {
	case 1:
		return fmt.Sprintf("%s%d%c", CSI, num[0], ctrl)
	case 2:
		return fmt.Sprintf("%s%d;%d%c", CSI, num[0], num[1], ctrl)
	}
	return ""
}

func getStdin(stdinChan chan<- string) {
	var err error
	for {
		input := make([]byte, StdinBuf)
		n, err := os.Stdin.Read(input)
		if err != nil {
			break
		}
		if n > 0 {
			stdinChan <- string(input[:n])
		}
	}
	close(stdinChan)
	log.Fatal(err)
}

func treatStdin(stdinChan <-chan string, chanCmd stdinToCmd, chanPrint stdinToPrint) {
	var wait bool
	var exit bool
	for stdin := range stdinChan {
		switch stdin {
		case "q":
			exit = !exit
			chanCmd.exit <- exit
		case "w":
			wait = !wait
			chanCmd.wait <- wait
		case "r":
			chanPrint.refresh <- true
		case CtrlD:
			chanPrint.head <- +10
		case CtrlU:
			chanPrint.head <- -10
		case DownKey:
			chanPrint.head <- +1
		case UpKey:
			chanPrint.head <- -1
		}
	}
}

func makeExecArray(cmdArray [][]string, length int) ([]*exec.Cmd, []*io.PipeReader, []*io.PipeWriter) {
	var execArray []*exec.Cmd
	var readArray []*io.PipeReader
	var writeArray []*io.PipeWriter

	for i := 0; i < length; i++ {
		switch len(cmdArray[i]) {
		case 1:
			execArray = append(execArray, exec.Command(cmdArray[i][0]))
		default:
			execArray = append(execArray, exec.Command(cmdArray[i][0], cmdArray[i][1:]...))
		}
		if i != 0 {
			execArray[i].Stdin = readArray[i-1]
		}
		if i < length-1 {
			read, write := io.Pipe()
			readArray = append(readArray, read)
			writeArray = append(writeArray, write)
			execArray[i].Stdout = writeArray[i]
		}
	}
	return execArray, readArray, writeArray
}

func runCmdArray(cmdArray [][]string) (string, error) {
	var out bytes.Buffer
	var err error
	var i int
	length := len(cmdArray)

	execArray, _, writeArray := makeExecArray(cmdArray, length)
	execArray[length-1].Stdout = &out

	for i = 0; i < length; i++ {
		err = execArray[i].Start()
		if err != nil {
			return "", err
		}
	}
	for i = 0; i < length-1; i++ {
		err = execArray[i].Wait()
		writeArray[i].Close()
		if err != nil {
			return "", err
		}
	}
	err = execArray[length-1].Wait()
	return out.String(), err
}

func runCmdRepeatedly(cmdstr []string, cmdoutChan chan<- string, chanCmd stdinToCmd, optCmd optToCmd) error {
	var cmdout string
	var cmdArray [][]string
	var err error
	var wait bool
	var exit bool

	sleepTime := time.Duration(optCmd.sleepSec * 1000) * time.Millisecond
	cmdArray = append(cmdArray, cmdstr)
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
		cmdout, err = runCmdArray(cmdArray)
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
	if line.length < height {
		return 0
	}
	head = head + dhead
	if head < 0 {
		head = 0
	} else if height+head > line.length {
		head = line.length - height
	}
	return head
}

func eraseToBegin(length int) {
	if length == 0 {
		return
	} else if length == 1 {
		fmt.Print(csiCode(Delete, All))
		fmt.Print(csiCode(Begin, 1))
	} else {
		for i := 0; i < length-1; i++ {
			fmt.Print(csiCode(Delete, All))
			fmt.Print(csiCode(Above, 1))
		}
	}
}

func moveToBegin(length int) {
	if length == 0 {
		return
	} else if length == 1 {
		fmt.Print(csiCode(Begin, 1))
	} else {
		fmt.Print(csiCode(Above, length-1))
	}
}

func wrapIn(width int, line string) string {
	if len(line) > width {
		return line[:width]
	} else {
		return line
	}
}

func coloring(color string, line string) string {
	return color + line + Normal
}

func printNew(i int, line strArray, print printFn) {
	fmt.Print(csiCode(Delete, All))
	print(wrapIn(line.width, line.elem[i]))
}

func printAsIs(i int, line strArray, print printFn) {
	if line.count[i] > 1 {
		fmt.Print(csiCode(Delete, All))
		print(line.colorElem[i])
	} else {
		fmt.Print(csiCode(Delete, All))
		print(wrapIn(line.width, line.elem[i]))
	}
}

func printChanges(i int, line strArray, print printFn) {
	if line.count[i] == CountMaxDef + 1 {
		fmt.Print(csiCode(Delete, All))
		print(line.colorElem[i])
	} else if line.count[i] == 1 {
		fmt.Print(csiCode(Delete, All))
		print(wrapIn(line.width, line.elem[i]))
	} else {
		fmt.Print(csiCode(Below, 1))
	}
}

func printLines(line strArray, head int, print printLine) {
	last := line.height + head - 1
	for i := head; i < last; i++ {
		print(i, line, fmt.Println)
	}
	print(last, line, fmt.Print)
}

func checkLineCount(line strArray, i int) int {
	if line.count[i] > 0 {
		return line.count[i]-1
	} else {
		return line.count[i]
	}
}

func checkChangeLine(oldLine strArray, line strArray) strArray {
	length := line.length
	if oldLine.length < length {
		length = oldLine.length
	}
	for i := 0; i < length; i++ {
		if oldLine.elem[i] == line.elem[i] {
			line.count[i] = checkLineCount(oldLine, i)
			if line.count[i] > 1 {
				line.colorElem[i] = oldLine.colorElem[i]
			}
		} else {
			line.colorElem[i] = coloring(Red, wrapIn(line.width, line.elem[i]))
			line.count[i] = CountMaxDef + 1
		}
	}
	return line
}

func printRepeatedly(cmdoutChan <-chan string, chanPrint stdinToPrint) {
	var oldLine strArray
	var line strArray
	var cmdout string
	var height, width int
	var head int

	for {
		winSize, err := ioctl.GetWinsize()
		if err != nil {
			log.Fatal(err)
		}
		height = int(winSize.Row) - 1
		width = int(winSize.Col)
		select {
		case refresh := <-chanPrint.refresh:
			if refresh {
				eraseToBegin(oldLine.height)
				printLines(oldLine, head, printNew)
			}
		case dHead := <-chanPrint.head:
			newHead := checkHead(oldLine, head, dHead, height)
			if newHead != head {
				head = newHead
				eraseToBegin(oldLine.height)
				printLines(oldLine, head, printAsIs)
			}
		case cmdout = <-cmdoutChan:
			line = newStrArray(cmdout, "\n", height, width)
			head = checkHead(line, head, 0, height)
			if oldLine.height != line.height {
				eraseToBegin(oldLine.height)
				printLines(line, head, printNew)
			} else {
				line = checkChangeLine(oldLine, line)
				moveToBegin(oldLine.height)
				printLines(line, head, printChanges)
			}
			oldLine = line
		}
	}
}

func main() {
	var interactive bool
	var shell bool
	var optCmd optToCmd
	var err error
	var ret int

	flag.Usage = func() {
		fmt.Printf("Usage: %s [-s sec] [-i] [-sh] [-f] command\n\n", os.Args[0])
		flag.PrintDefaults()
	}
	flag.Float64Var(&optCmd.sleepSec, "s", SleepSecDef, "sleep second")
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
	chanPrint := stdinToPrint{make(chan bool), make(chan int)}

	err = ioctl.SetOrgTermios()
	if err != nil {
		log.Fatal(err)
	}

	if !interactive {
		err = ioctl.ChangeTermiosLflag(^(ioctl.ECHO|ioctl.ICANNON))
		if err != nil {
			log.Fatal(err)
		}
		stdinChan := make(chan string)
		go treatStdin(stdinChan, chanCmd, chanPrint)
		go getStdin(stdinChan)
	}

	cmdoutChan := make(chan string)
	go printRepeatedly(cmdoutChan, chanPrint)
	err = runCmdRepeatedly(cmd, cmdoutChan, chanCmd, optCmd)
	if err != nil {
		ret = 1
	}
	err = ioctl.ResetTermiosLflag()
	if err != nil {
		log.Fatal(err)
	}
	os.Exit(ret)
}
