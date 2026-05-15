package rss

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestPushLimiter_Unlimited(t *testing.T) {
	limiter := NewPushLimiter(0)
	for i := 0; i < 100; i++ {
		require.True(t, limiter.Allow("sub1"), "unlimited should always allow")
	}
}

func TestPushLimiter_RateLimit(t *testing.T) {
	limiter := NewPushLimiter(3)

	require.True(t, limiter.Allow("sub1"))
	require.True(t, limiter.Allow("sub1"))
	require.True(t, limiter.Allow("sub1"))
	require.False(t, limiter.Allow("sub1"), "should be rate limited")
}

func TestPushLimiter_PerSubscription(t *testing.T) {
	limiter := NewPushLimiter(2)

	require.True(t, limiter.Allow("sub1"))
	require.True(t, limiter.Allow("sub1"))
	require.False(t, limiter.Allow("sub1"))

	require.True(t, limiter.Allow("sub2"), "different sub should be independent")
}

func TestPushLimiter_Reset(t *testing.T) {
	limiter := NewPushLimiter(1)

	require.True(t, limiter.Allow("sub1"))
	require.False(t, limiter.Allow("sub1"))

	limiter.Reset("sub1")
	require.True(t, limiter.Allow("sub1"), "should allow after reset")
}
