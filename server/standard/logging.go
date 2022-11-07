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

package standard

import (
	grpc_zap "github.com/grpc-ecosystem/go-grpc-middleware/logging/zap"
	"github.com/streamingfast/logging"
)

var zlog, _ = logging.PackageLogger("dgrpc", "github.com/streamingfast/dgrpc")
var zlogGRPC, _ = logging.PackageLogger("dgrpc", "github.com/streamingfast/dgrpc/internal_grpc")

func init() {
	// Level 0 verbosity in grpc-go less chatty
	// https://github.com/grpc/grpc-go/blob/master/Documentation/log_levels.md
	grpc_zap.ReplaceGrpcLoggerV2(zlogGRPC)
}
