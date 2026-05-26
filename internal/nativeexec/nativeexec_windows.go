//go:build windows && cgo

package nativeexec

/*
#include <process.h>
#include <stdlib.h>

static int run_wait(char* path, char** argv) {
	return _spawnv(_P_WAIT, path, (const char* const*)argv);
}
*/
import "C"

import (
	"fmt"
	"unsafe"
)

func Run(binary string, args []string) ([]byte, error) {
	argv := make([]*C.char, 0, len(args)+2)

	cbin := C.CString(binary)
	defer C.free(unsafe.Pointer(cbin))
	argv = append(argv, cbin)

	// Free each string slice arg inside a helper function to avoid defer-in-loop resource leaks
	cargs := make([]*C.char, 0, len(args))
	defer func() {
		for _, carg := range cargs {
			C.free(unsafe.Pointer(carg))
		}
	}()

	for _, arg := range args {
		carg := C.CString(arg)
		cargs = append(cargs, carg)
		argv = append(argv, carg)
	}
	argv = append(argv, nil)

	code := C.run_wait(cbin, (**C.char)(unsafe.Pointer(&argv[0])))
	if int(code) == -1 {
		return nil, fmt.Errorf("native spawn failed")
	}
	if int(code) != 0 {
		return nil, fmt.Errorf("exit status %d", int(code))
	}
	return nil, nil
}
