package ethspam

import (
	"io"
	"math/rand"
)

// FIXME: Might make sense to make this a standalone package that just generates
// various kinds of eth request spam to stdout?

var jsons = []string{
	`{"id":1,"jsonrpc":"2.0","method":"eth_getBlockByNumber","params":["latest", false]}`,
}

type Random struct{}

func (r Random) Read(p []byte) (n int, err error) {
	if len(jsons) == 0 {
		return 0, io.EOF
	}
	p = []byte(jsons[rand.Int()%len(jsons)])
	return len(p), nil
}
