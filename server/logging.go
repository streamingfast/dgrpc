// Copyright 2019 dfuse Platform Inc.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package server

import (
	"log"
	"regexp"
	"strings"

	"github.com/streamingfast/logging"
	"go.uber.org/zap"
)

var zlog, _ = logging.PackageLogger("dgrpc", "github.com/streamingfast/dgrpc/server")

// NewHTTPErrorLogger creates a *log.Logger suitable for http.Server.ErrorLog.
// All messages are routed to the given zap logger at error level, except those
// matching one of the suppressPatterns, which are silently dropped.
func NewHTTPErrorLogger(logger *zap.Logger, suppressPatterns []*regexp.Regexp) (*log.Logger, error) {
	if len(suppressPatterns) == 0 {
		return zap.NewStdLogAt(logger, zap.ErrorLevel)
	}
	return log.New(&filteredHTTPErrorWriter{logger: logger, suppress: suppressPatterns}, "", 0), nil
}

type filteredHTTPErrorWriter struct {
	logger   *zap.Logger
	suppress []*regexp.Regexp
}

func (w *filteredHTTPErrorWriter) Write(p []byte) (n int, err error) {
	msg := strings.TrimRight(string(p), "\n")
	for _, re := range w.suppress {
		if re.MatchString(msg) {
			return len(p), nil
		}
	}
	w.logger.Error(msg)
	return len(p), nil
}
