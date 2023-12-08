package ctl

import "fmt"

type Err string

func (e Err) Error() string {
	return string(e)
}

func Errf(str string, a ...any) Err {
	return Err(fmt.Sprintf(str, a...))
}
