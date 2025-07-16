package server

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

// TestIsolateRequestPanicUnaryInterceptor tests that panics in unary gRPC handlers are properly recovered
func TestIsolateRequestPanicUnaryInterceptor(t *testing.T) {
	logger := zap.NewNop()
	
	// Create the interceptor
	interceptor := IsolateRequestPanicUnaryInterceptor(logger)
	
	// Create a mock handler that panics
	panicHandler := func(ctx context.Context, req interface{}) (interface{}, error) {
		panic("simulated panic in unary handler")
	}
	
	// Create mock server info
	info := &grpc.UnaryServerInfo{
		FullMethod: "/test.TestService/TestUnary",
	}
	
	// Call the interceptor with the panicking handler
	resp, err := interceptor(context.Background(), "test-request", info, panicHandler)
	
	// Verify that the panic was recovered and converted to a gRPC error
	require.Error(t, err)
	assert.Nil(t, resp)
	
	st, ok := status.FromError(err)
	require.True(t, ok, "Expected gRPC status error")
	assert.Equal(t, codes.Internal, st.Code())
	assert.Contains(t, st.Message(), "internal server error")
	assert.Contains(t, st.Message(), "simulated panic in unary handler")
}

// TestIsolateRequestPanicUnaryInterceptorNormalFlow tests that normal requests pass through unchanged
func TestIsolateRequestPanicUnaryInterceptorNormalFlow(t *testing.T) {
	logger := zap.NewNop()
	
	// Create the interceptor
	interceptor := IsolateRequestPanicUnaryInterceptor(logger)
	
	// Create a normal handler that doesn't panic
	normalHandler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return "test-response", nil
	}
	
	// Create mock server info
	info := &grpc.UnaryServerInfo{
		FullMethod: "/test.TestService/TestUnary",
	}
	
	// Call the interceptor with the normal handler
	resp, err := interceptor(context.Background(), "test-request", info, normalHandler)
	
	// Verify that the normal flow is unchanged
	require.NoError(t, err)
	assert.Equal(t, "test-response", resp)
}

// TestIsolateRequestPanicStreamInterceptor tests that panics in stream gRPC handlers are properly recovered
func TestIsolateRequestPanicStreamInterceptor(t *testing.T) {
	logger := zap.NewNop()
	
	// Create the interceptor
	interceptor := IsolateRequestPanicStreamInterceptor(logger)
	
	// Create a mock handler that panics
	panicHandler := func(srv interface{}, stream grpc.ServerStream) error {
		panic("simulated panic in stream handler")
	}
	
	// Create mock server info
	info := &grpc.StreamServerInfo{
		FullMethod: "/test.TestService/TestStream",
	}
	
	// Call the interceptor with the panicking handler
	err := interceptor(nil, &mockServerStream{}, info, panicHandler)
	
	// Verify that the panic was recovered and converted to a gRPC error
	require.Error(t, err)
	
	st, ok := status.FromError(err)
	require.True(t, ok, "Expected gRPC status error")
	assert.Equal(t, codes.Internal, st.Code())
	assert.Contains(t, st.Message(), "internal server error")
	assert.Contains(t, st.Message(), "simulated panic in stream handler")
}

// TestIsolateRequestPanicStreamInterceptorNormalFlow tests that normal streaming requests pass through unchanged
func TestIsolateRequestPanicStreamInterceptorNormalFlow(t *testing.T) {
	logger := zap.NewNop()
	
	// Create the interceptor
	interceptor := IsolateRequestPanicStreamInterceptor(logger)
	
	// Create a normal handler that doesn't panic
	normalHandler := func(srv interface{}, stream grpc.ServerStream) error {
		return nil
	}
	
	// Create mock server info
	info := &grpc.StreamServerInfo{
		FullMethod: "/test.TestService/TestStream",
	}
	
	// Call the interceptor with the normal handler
	err := interceptor(nil, &mockServerStream{}, info, normalHandler)
	
	// Verify that the normal flow is unchanged
	require.NoError(t, err)
}

// mockServerStream is a minimal mock implementation of grpc.ServerStream for testing
type mockServerStream struct{}

func (m *mockServerStream) SetHeader(md metadata.MD) error  { return nil }
func (m *mockServerStream) SendHeader(md metadata.MD) error { return nil }
func (m *mockServerStream) SetTrailer(md metadata.MD)       {}
func (m *mockServerStream) Context() context.Context       { return context.Background() }
func (m *mockServerStream) SendMsg(msg interface{}) error  { return nil }
func (m *mockServerStream) RecvMsg(msg interface{}) error  { return nil }