package decimal

import (
	"bytes"
	"io"
)

type buffer struct{ bytes.Buffer }

func (b *buffer) String() string {
	// Trim zeros.
	buf := b.Bytes()
	i := len(buf) - 1
	for ; i >= 0 && buf[i] == '0'; i-- {
	}
	if buf[i] == '.' {
		i--
	}
	b.Truncate(i + 1)
	return b.Buffer.String()
}

type writer interface {
	io.Writer
	io.ByteWriter
	WriteString(string) (int, error)

	// Change this to fmt.Stringer once we import fmt
	// to make the Format method.
	String() string
}
