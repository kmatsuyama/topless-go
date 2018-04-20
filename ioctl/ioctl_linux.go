package ioctl

import (
	"golang.org/x/sys/unix"
	"os"
)

const (
	ECHO = uint32(unix.ECHO)
	ICANNON = uint32(unix.ICANON)
)

var (
	orgLflag uint32
)

func GetWinsize() (*unix.Winsize, error){
	return unix.IoctlGetWinsize(int(os.Stdout.Fd()), unix.TIOCGWINSZ)
}

func SetOrgTermios() error {
	termios, err := unix.IoctlGetTermios(int(os.Stdout.Fd()), unix.TCGETS)
	if err != nil {
		return err
	}
	orgLflag = termios.Lflag
	return err
}

func ChangeTermiosLflag(flag uint32) error {
	termios, err := unix.IoctlGetTermios(int(os.Stdout.Fd()), unix.TCGETS)
	if err != nil {
		return err
	}
	termios.Lflag &= flag
	return unix.IoctlSetTermios(int(os.Stdout.Fd()), unix.TCSETS, termios)
}

func ResetTermiosLflag() error {
	termios, err := unix.IoctlGetTermios(int(os.Stdout.Fd()), unix.TCGETS)
	if err != nil {
		return err
	}
	termios.Lflag = orgLflag
	return unix.IoctlSetTermios(int(os.Stdout.Fd()), unix.TCSETS, termios)
}
