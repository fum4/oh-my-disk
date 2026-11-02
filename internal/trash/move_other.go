//go:build !darwin || !cgo

package trash

import "fmt"

func move(string, uint64, uint64) error {
	return fmt.Errorf("restorable Trash requires macOS with cgo; no files moved")
}
