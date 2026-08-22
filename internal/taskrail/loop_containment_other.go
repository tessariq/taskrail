//go:build !(aix || darwin || dragonfly || freebsd || linux || netbsd || openbsd || solaris || windows)

package taskrail

import "errors"

func newLoopChildContainment() (loopChildContainment, error) {
	return nil, errors.New("loop process containment is unsupported on this platform")
}
