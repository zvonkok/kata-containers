// Copyright (c) 2017 Intel Corporation
//
// SPDX-License-Identifier: Apache-2.0
//

package vcmock

import (
	"runtime"
	"strings"
)

// getSelf returns the name of the _calling_ function
func getSelf() string {
	pc := make([]uintptr, 1)

	// return the program counter for the calling function
	runtime.Callers(2, pc)

	f := runtime.FuncForPC(pc[0])
	return f.Name()
}

// IsMockError returns true if the specified error was generated by this
// package.
func IsMockError(err error) bool {
	if err == nil {
		return false
	}
	return strings.HasPrefix(err.Error(), mockErrorPrefix)
}
