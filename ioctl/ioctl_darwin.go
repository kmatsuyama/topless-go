package ioctl

import (
	"golang.org/x/sys/unix"
	"os"
)

const (
	ECHO = uint64(unix.ECHO)
	ICANNON = uint64(unix.ICANON)
)

var (
	orgLflag uint64
)

func GetWinsize() (*unix.Winsize, error){
	return unix.IoctlGetWinsize(int(os.Stdout.Fd()), unix.TIOCGWINSZ)
}

func SetOrgTermios() error {
	termios, err := unix.IoctlGetTermios(int(os.Stdout.Fd()), unix.TIOCGETA)
	if err != nil {
		return err
	}
	orgLflag = termios.Lflag
	return err
}

func ChangeTermiosLflag(flag uint64) error {
	termios, err := unix.IoctlGetTermios(int(os.Stdout.Fd()), unix.TIOCGETA)
	if err != nil {
		return err
	}
	termios.Lflag &= flag
	return unix.IoctlSetTermios(int(os.Stdout.Fd()), unix.TIOCSETA, termios)
}

func ResetTermiosLflag() error {
	termios, err := unix.IoctlGetTermios(int(os.Stdout.Fd()), unix.TIOCGETA)
	if err != nil {
		return err
	}
	termios.Lflag = orgLflag
	return unix.IoctlSetTermios(int(os.Stdout.Fd()), unix.TIOCSETA, termios)
}
