//go:build !linux

package secretscmd

import "errors"

func runFile(_ []byte, _ []string) error {
 return errors.New("secrets exec-file requires Linux memfd")
}
