//go:build darwin && cgo

package trash

/*
#cgo LDFLAGS: -framework Foundation
#include <stdlib.h>
char *omd_trash(const char *, unsigned long long, unsigned long long);
*/
import "C"
import (
	"fmt"
	"unsafe"
)

func move(path string, dev, ino uint64) error {
	p := C.CString(path)
	defer C.free(unsafe.Pointer(p))
	err := C.omd_trash(p, C.ulonglong(dev), C.ulonglong(ino))
	if err != nil {
		defer C.free(unsafe.Pointer(err))
		return fmt.Errorf("%s", C.GoString(err))
	}
	return nil
}
