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
	"./stdout"
)

const (
	StdinBuf = 128
	SleepSecDef = 1
	CountMaxDef = 3
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

func printRepeatedly(cmdoutChan <-chan string, chanPrint stdinToPrint) {
	var oldLine stdout.StrArray
	var line stdout.StrArray
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
				stdout.EraseUp(oldLine.Height)
				stdout.Lines(oldLine, head, stdout.New)
			}
		case dHead := <-chanPrint.head:
			newHead := stdout.CheckHead(oldLine, head, dHead, height)
			if newHead != head {
				head = newHead
				stdout.EraseUp(oldLine.Height)
				stdout.Lines(oldLine, head, stdout.AsIs)
			}
		case cmdout = <-cmdoutChan:
			line = stdout.NewStrArray(cmdout, "\n", height, width)
			head = stdout.CheckHead(line, head, 0, height)
			if oldLine.Height != line.Height {
				stdout.EraseUp(oldLine.Height)
				stdout.Lines(line, head, stdout.New)
			} else {
				line = stdout.CheckChange(oldLine, line)
				stdout.MoveUp(oldLine.Height)
				stdout.Lines(line, head, stdout.Changes)
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
