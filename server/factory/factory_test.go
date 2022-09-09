package factory

import (
	"net/url"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestUrl(t *testing.T) {

	u, err := url.Parse("traffic-director://?client-only=true")
	require.NoError(t, err)
	require.Equal(t, "traffic-director", u.Scheme)

}
