package io

import "io"

func CloseSilent(closer io.Closer) { _ = closer.Close() }
func CloseSilentWithErrorHandler(closer io.Closer, handleError func(source interface{}, err error)) {
	err := closer.Close()
	if err != nil {
		handleError(&closer, err)
	}
}
