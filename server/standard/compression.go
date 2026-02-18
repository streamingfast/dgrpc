package standard

import (
	"fmt"
	"net/http"
	"strings"

	"go.uber.org/zap"
	"google.golang.org/grpc/encoding"
)

func CompressionHandler(enforceCompression bool, h http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		compressor := "identity"

		vv := r.Header.Values("grpc-encoding")
		if len(vv) > 0 {
			if len(vv) != 1 {
				writeBadRequest(w, fmt.Sprintf("grpc-encoding should be a single value, got: %v", vv))
				return
			}
			compressor = strings.TrimSpace(vv[0])
			if compressor != "" {
				if encoding.GetCompressor(compressor) == nil {
					//encoding specifically selected by user is not supported
					writeBadRequest(w, fmt.Sprintf("grpc-encoding %v is not supported", compressor))
					return
				}

				zlog.Info("compression enabled", zap.String("grpc-encoding", compressor))
				h.ServeHTTP(w, r)
				return
			}
		}

		if c := firstSupportedCompressor(r.Header.Values("grpc-accept-encoding")); c != "" {
			compressor = strings.TrimSpace(c)
		} else if c := firstSupportedCompressor(r.Header.Values("connect-accept-encoding")); c != "" {
			compressor = strings.TrimSpace(c)
		} else if c := firstSupportedCompressor(r.Header.Values("accept-encoding")); c != "" {
			compressor = strings.TrimSpace(c)
		} else {
			if enforceCompression {
				writeBadRequest(w, "no supported compression found.")
				return
			}
		}

		zlog.Info("compression enabled", zap.String("grpc-encoding", compressor))
		r.Header.Add("grpc-encoding", compressor)
		h.ServeHTTP(w, r)

	})
}

func firstSupportedCompressor(compressors []string) string {
	for _, cs := range compressors {
		for _, c := range strings.Split(cs, ",") {
			if encoding.GetCompressor(strings.TrimSpace(c)) != nil {
				return c
			}
		}
	}
	return ""
}

func writeBadRequest(w http.ResponseWriter, message string) {
	w.WriteHeader(http.StatusBadRequest)
	w.Write([]byte(message))
}
