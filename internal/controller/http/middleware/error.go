package middleware

import (
	"context"
	"net/http"

	"github.com/go-faster/jx"
	"github.com/ogen-go/ogen/ogenerrors"
)

// ErrorHandler is a middleware for handling ogen errors.
func ErrorHandler(_ context.Context, w http.ResponseWriter, _ *http.Request, err error) {
	code := ogenerrors.ErrorCode(err)

	w.Header().Set("Content-Type", "application/problem+json")
	w.WriteHeader(code)

	e := jx.GetEncoder()
	e.ObjStart()

	e.FieldStart("title")
	e.StrEscape(http.StatusText(code))

	e.FieldStart("status")
	e.Int(code)

	e.FieldStart("detail")
	e.StrEscape(err.Error())

	e.ObjEnd()

	_, _ = w.Write(e.Bytes())
}
