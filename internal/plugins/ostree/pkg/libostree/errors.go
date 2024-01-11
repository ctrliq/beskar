package ostree

type Err string

func (e Err) Error() string {
	return string(e)
}

const (
	ErrInvalidPath = Err("invalid path")
)
