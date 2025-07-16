package connectrpc

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

// TestIsolateRequestPanicInterceptorCreation tests that the interceptor can be created successfully
func TestIsolateRequestPanicInterceptorCreation(t *testing.T) {
	logger := zap.NewNop()
	
	// Create the interceptor
	interceptor := IsolateRequestPanicInterceptor(logger)
	
	// Verify that the interceptor is created successfully
	require.NotNil(t, interceptor)
	assert.NotNil(t, interceptor.WrapUnary)
}

// TestPanicRecoveryMetricsInitialization tests that metrics are properly initialized
func TestPanicRecoveryMetricsInitialization(t *testing.T) {
	// Verify that the metrics are initialized
	require.NotNil(t, connectPanicRecoveryMetrics)
	require.NotNil(t, connectPanicCounter)
}