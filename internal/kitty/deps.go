package kitty

import (
	"io"
	"os"
)

type Deps struct {
	Stdin          io.Reader
	Stdout         io.Writer
	Stderr         io.Writer
	LookupEnv      func(string) (string, bool)
	RunCommand     func(string, ...string) ([]byte, error)
	FileExists     func(string) (bool, error)
	RemoveFile     func(string) error
	ReadFile       func(string) ([]byte, error)
	ReadDir        func(string) ([]os.DirEntry, error)
	WriteFile      func(string, []byte, os.FileMode) error
	MkdirAll       func(string, os.FileMode) error
	ExecutablePath func() (string, error)
	Getwd          func() (string, error)
	UserHomeDir    func() (string, error)
}
