package standard

import (
	"fmt"
	"net/http"

	"google.golang.org/grpc/encoding"
)

func compressionHandler(enforceCompression bool, h http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		compressor := "identity"

		vv := r.Header.Values("grpc-encoding")
		if len(vv) > 0 {
			if len(vv) != 1 {
				writeBadRequest(w, fmt.Sprintf("grpc-encoding should be a single value, got: %v", vv))
				return
			}
			compressor = vv[0]
			if compressor != "" {
				if encoding.GetCompressor(compressor) == nil {
					//encoding specifically selected by user is not supported
					writeBadRequest(w, fmt.Sprintf("grpc-encoding %v is not supported", compressor))
					return
				}

				h.ServeHTTP(w, r)
				return
			}
		}

		if c := firstSupportedCompressor(r.Header.Values("grpc-accept-encoding")); c != "" {
			compressor = c
		} else if c := firstSupportedCompressor(r.Header.Values("connect-accept-encoding")); c != "" {
			compressor = c
		} else if c := firstSupportedCompressor(r.Header.Values("accept-encoding")); c != "" {
			compressor = c
		} else {
			if enforceCompression {
				writeBadRequest(w, "no supported compression found.")
				return
			}
		}

		r.Header.Add("grpc-encoding", compressor)
		h.ServeHTTP(w, r)

	})
}

func firstSupportedCompressor(compressors []string) string {
	for _, c := range compressors {
		if encoding.GetCompressor(c) != nil {
			return c
		}
	}
	return ""
}

func writeBadRequest(w http.ResponseWriter, message string) {
	w.WriteHeader(http.StatusBadRequest)
	w.Write([]byte(message))
}

//var compressionHeader = map[string]map[string]bool{
//	"grpc-accept-encoding":    {"gzip": true, "zstd": true},
//	"connect-accept-encoding": {"gzip": true, "zstd": true},
//	"accept-encoding":         {"gzip": true}, // HTTP encoding for connect+proto in browser
//}

//
//func compressorsFromHeader(header http.Header) (out map[string]bool) {
//	out = make(map[string]bool)
//	for k, v := range header {
//		petitK := strings.ToLower(k)
//		if petitK == "grpc-accept-encoding" || petitK == "connect-accept-encoding" || petitK == "accept-encoding" {
//			for _, vv := range v {
//				for _, vvv := range strings.Split(vv, ",") {
//					out[strings.ToLower(vvv)] = true
//				}
//			}
//		}
//	}
//	return
//}
