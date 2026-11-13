// Command hello is a throwaway program for testing tools/winres.
//
// The winres tests build it for Windows together with a freshly generated
// rsrc_windows_amd64.syso and then inspect the executable's resources. When
// the program itself runs on Windows (or under Wine) it additionally asks the
// operating system for those resources, see selfcheck_windows.go.
package main

import (
	"fmt"
	"os"
)

func main() {
	fmt.Println("hello from the Drifthold resource test program")
	if err := selfCheck(); err != nil {
		fmt.Println("selfcheck: FAILED:", err)
		os.Exit(1)
	}
}
