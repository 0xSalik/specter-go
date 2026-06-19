package dashboard

import (
	"fmt"
	"io"
	"strconv"
)

func parsePerms(s string) int64 {
	v, err := strconv.ParseInt(s, 10, 64)
	if err != nil {
		return 0
	}
	return v
}

func errStatus(code int) error {
	return fmt.Errorf("discord API returned status %d", code)
}

func readAll(r io.Reader) ([]byte, error) {
	return io.ReadAll(io.LimitReader(r, 8<<20))
}
