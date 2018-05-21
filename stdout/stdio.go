package stdout

import (
	"fmt"
	"strings"
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
	Normal  = "\x1B[0m"
	Red     = "\x1B[31m"
	Green   = "\x1B[32m"
	Yellow  = "\x1B[33m"
	Blue    = "\x1B[34m"
	Magenta = "\x1B[35m"
	Cyan    = "\x1B[36m"
	White   = "\x1B[37m"
	BgRed     = "\x1B[41m"
	BgGreen   = "\x1B[42m"
	BgYellow  = "\x1B[43m"
	BgBlue    = "\x1B[44m"
	BgMagenta = "\x1B[45m"
	BgCyan    = "\x1B[46m"
	BgWhite   = "\x1B[47m"
	RedB     = "\x1B[31;1m"
	GreenB   = "\x1B[32;1m"
	YellowB  = "\x1B[33;1m"
	BlueB    = "\x1B[34;1m"
	MagentaB = "\x1B[35;1m"
	CyanB    = "\x1B[36;1m"
	WhiteB   = "\x1B[37;1m"
	Red_     = "\x1B[31;4m"
	Green_   = "\x1B[32;4m"
	Yellow_  = "\x1B[33;4m"
	Blue_    = "\x1B[34;4m"
	Magenta_ = "\x1B[35;4m"
	Cyan_    = "\x1B[36;4m"
	White_   = "\x1B[37;1m"
)

const (
	CSI = "\033["
)

const (
	Before = 0
	After  = 1
	All    = 2
)

type StrArray struct {
	elem      []string
	colorElem []string
	length    int
	height    int
	width     int
	count     []int
	countMax  int
}

type printFn func(...interface{}) (n int, err error)
type printLine func(int, StrArray, printFn) ()

func NewStrArray(str string, delim string, height int, width int, countMax int) StrArray {
	elem := strings.Split(str, delim)
	length := len(elem)
	if length < height {
		height = length
	}
	return StrArray{
		elem:   elem,
		colorElem:  make([]string, length),
		length: length,
		height: height,
		width: width,
		count: make([]int, length),
		countMax: countMax,
	}
}

func IsSameHeight(oldLine StrArray, line StrArray) bool {
	return oldLine.height == line.height
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

func Erase(line StrArray) {
	eraseUp(line.height)
}

func eraseUp(length int) {
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

func BackToTop(line StrArray) {
	moveUp(line.height)
}

func moveUp(length int) {
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

func New(i int, line StrArray, printF printFn) {
	fmt.Print(csiCode(Delete, All))
	printF(wrapIn(line.width, line.elem[i]))
}

func AsIs(i int, line StrArray, printF printFn) {
	if line.count[i] > 1 {
		fmt.Print(csiCode(Delete, All))
		printF(line.colorElem[i])
	} else {
		fmt.Print(csiCode(Delete, All))
		printF(wrapIn(line.width, line.elem[i]))
	}
}

func Changes(i int, line StrArray, printF printFn) {
	if line.count[i] == line.countMax + 1 {
		fmt.Print(csiCode(Delete, All))
		printF(line.colorElem[i])
	} else if line.count[i] == 1 {
		fmt.Print(csiCode(Delete, All))
		printF(wrapIn(line.width, line.elem[i]))
	} else {
		fmt.Print(csiCode(Below, 1))
	}
}

func Lines(line StrArray, head int, printL printLine) {
	last := line.height + head - 1
	for i := head; i < last; i++ {
		printL(i, line, fmt.Println)
	}
	printL(last, line, fmt.Print)
}

func min(a, b int) int {
   if a > b {
      return b
   }
   return a
}

func coloring(color string, line string) string {
	return color + line + Normal
}

func colorDiff(orgColor string, color string, oldLine string, line string) string {
	var same bool

	colorStr := coloring(orgColor, line)
	num := len(orgColor)

	for i := 0; i < min(len(oldLine), len(line)); i++ {
		if line[i] == oldLine[i] && !same {
			colorStr = colorStr[:num] + Normal + orgColor + colorStr[num:]
			num += len(Normal + orgColor)
			same = true
		} else if line[i] != oldLine[i] && same {
			colorStr = colorStr[:num] + color + colorStr[num:]
			num += len(color)
			same = false
		}
		num++
	}
	return colorStr
}

func CheckHead(line StrArray, head int, dhead int, height int) int {
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

func checkLineCount(line StrArray, i int) int {
	if line.count[i] > 0 {
		return line.count[i]-1
	} else {
		return line.count[i]
	}
}

func CheckChange(oldLine StrArray, line StrArray) StrArray {
	for i := 0; i < min(oldLine.length, line.length); i++ {
		if oldLine.elem[i] == line.elem[i] {
			line.count[i] = checkLineCount(oldLine, i)
			if line.count[i] > 1 {
				line.colorElem[i] = oldLine.colorElem[i]
			}
		} else {
			line.colorElem[i] = colorDiff(Red, Red_, wrapIn(oldLine.width, oldLine.elem[i]),  wrapIn(line.width, line.elem[i]))
			line.count[i] = line.countMax + 1
		}
	}
	return line
}
