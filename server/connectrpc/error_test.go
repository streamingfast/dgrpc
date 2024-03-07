package connectrpc

import (
	"fmt"
	"testing"

	"connectrpc.com/connect"
	"github.com/stretchr/testify/assert"
	"go.uber.org/zap/zapcore"
)

func Test_obfuscateError(t *testing.T) {
	tests := []struct {
		name      string
		err       error
		expectErr error
	}{
		{"basic error", fmt.Errorf("basic error"), connect.NewError(connect.CodeUnknown, fmt.Errorf("Unexpected error. Please try later."))},
		{"unauthenticated", connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("invalid JWT signature")), connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("invalid JWT signature"))},
		{"connect web internal", connect.NewError(connect.CodeInternal, fmt.Errorf("asdfasdf")), connect.NewError(connect.CodeInternal, fmt.Errorf("Unexpected error. Please try later."))},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			i := &ErrorsInterceptor{}
			assert.Equal(t, test.expectErr, i.obfuscateError(test.err))
		})
	}
}

func Test_logError(t *testing.T) {
	tests := []struct {
		error  *connect.Error
		expect zapcore.Level
	}{
		{connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("invalid JWT signature")), zapcore.InfoLevel},
		{connect.NewError(connect.CodeInternal, fmt.Errorf("internal foor")), zapcore.ErrorLevel},
		{connect.NewError(connect.CodeUnknown, fmt.Errorf("unexpected error. Please try later.")), zapcore.ErrorLevel},
		{connect.NewError(connect.CodeNotFound, fmt.Errorf("not found")), zapcore.InfoLevel},
	}

	for _, test := range tests {
		t.Run("", func(t *testing.T) {
			assert.Equal(t, test.expect, getLogSeverity(test.error))
		})
	}
}
