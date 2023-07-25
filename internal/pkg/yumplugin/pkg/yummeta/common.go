package yummeta

type XMLRoot interface {
	Data() []byte
	Href(string)
}
