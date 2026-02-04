package standard

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"google.golang.org/grpc/encoding"
	_ "google.golang.org/grpc/encoding/gzip"
)

func TestCompressionHandler(t *testing.T) {
	// Ensure gzip is registered (imported above)
	assert.NotNil(t, encoding.GetCompressor("gzip"), "gzip compressor should be registered")

	tests := []struct {
		name               string
		headers            map[string][]string
		expectedStatus     int
		expectedBody       string
		expectedOutEncoded string
	}{
		{
			name: "valid grpc-encoding",
			headers: map[string][]string{
				"grpc-encoding": {"gzip"},
			},
			expectedStatus:     http.StatusOK,
			expectedOutEncoded: "gzip",
		},
		{
			name: "multiple grpc-encoding values",
			headers: map[string][]string{
				"grpc-encoding": {"gzip", "identity"},
			},
			expectedStatus: http.StatusBadRequest,
			expectedBody:   "grpc-encoding should be a single value, got: [gzip identity]",
		},
		{
			name: "unsupported grpc-encoding",
			headers: map[string][]string{
				"grpc-encoding": {"unsupported"},
			},
			expectedStatus: http.StatusBadRequest,
			expectedBody:   "grpc-encoding unsupported is not supported",
		},
		{
			name: "fallback to grpc-accept-encoding",
			headers: map[string][]string{
				"grpc-accept-encoding": {"unsupported", "gzip"},
			},
			expectedStatus:     http.StatusOK,
			expectedOutEncoded: "gzip",
		},
		{
			name: "fallback to connect-accept-encoding",
			headers: map[string][]string{
				"connect-accept-encoding": {"gzip"},
			},
			expectedStatus:     http.StatusOK,
			expectedOutEncoded: "gzip",
		},
		{
			name: "fallback to accept-encoding",
			headers: map[string][]string{
				"accept-encoding": {"gzip"},
			},
			expectedStatus:     http.StatusOK,
			expectedOutEncoded: "gzip",
		},
		{
			name: "no supported compression found",
			headers: map[string][]string{
				"accept-encoding": {"unsupported"},
			},
			expectedStatus: http.StatusBadRequest,
			expectedBody:   "no supported compression found.",
		},
		{
			name:           "no compression headers",
			headers:        map[string][]string{},
			expectedStatus: http.StatusBadRequest,
			expectedBody:   "no supported compression found.",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if tt.expectedOutEncoded != "" {
					assert.Equal(t, tt.expectedOutEncoded, r.Header.Get("grpc-encoding"))
				}
				w.WriteHeader(http.StatusOK)
			})

			h := compressionHandler(true, handler)

			req := httptest.NewRequest("POST", "/", nil)
			for k, vv := range tt.headers {
				for _, v := range vv {
					req.Header.Add(k, v)
				}
			}

			rr := httptest.NewRecorder()
			h.ServeHTTP(rr, req)

			assert.Equal(t, tt.expectedStatus, rr.Code)
			if tt.expectedBody != "" {
				assert.Equal(t, tt.expectedBody, rr.Body.String())
			}
		})
	}
}

func TestFirstSupportedCompressor(t *testing.T) {
	// Ensure gzip is registered
	assert.NotNil(t, encoding.GetCompressor("gzip"))

	tests := []struct {
		name        string
		compressors []string
		expected    string
	}{
		{
			name:        "first is supported",
			compressors: []string{"gzip", "identity"},
			expected:    "gzip",
		},
		{
			name:        "second is supported",
			compressors: []string{"unsupported", "gzip"},
			expected:    "gzip",
		},
		{
			name:        "none supported",
			compressors: []string{"unsupported", "none"},
			expected:    "",
		},
		{
			name:        "empty list",
			compressors: []string{},
			expected:    "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, firstSupportedCompressor(tt.compressors))
		})
	}
}
