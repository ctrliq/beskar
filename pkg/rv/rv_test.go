package rv

import "math/rand"

const letterBytes = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ/-_"

var randLetter = rand.New(rand.NewSource(123456789))

func RandStringBytes(n int) string {
	b := make([]byte, n)
	for i := range b {
		b[i] = letterBytes[randLetter.Intn(len(letterBytes))]
	}
	return string(b)
}
