package errors

import (
	"bytes"
	"fmt"
)

const (
	EINTERNAL = "internal"
	ECONFLICT = "conflict"
	EINVALID  = "invalid"
	ENOTFOUND = "not_found"
)

type Error struct {
	// Machine-readable error code
	Code string

	// Human-readable error message
	Message string

	// Logical operator
	Op string

	// Nested Error
	Err error
}

func (e *Error) Error() string {
	var buf bytes.Buffer
	if e.Op != "" {
		fmt.Fprintf(&buf, "%s:", e.Op)
	}
	if e.Err != nil {
		buf.WriteString(e.Err.Error())
	} else {
		if e.Code != "" {
			fmt.Fprintf(&buf, "<%s>", e.Code)
		}
		buf.WriteString(e.Message)
	}
	return buf.String()
}

func ErrorCode(err error) string {
	if err == nil {
		return ""
	} else if e, ok := err.(*Error); ok && e.Code != "" {
		return e.Code
	} else if ok && e.Err != nil {
		return ErrorCode(e.Err)
	}
	return EINTERNAL
}
