package instance_test

import (
	"testing"

	"github.com/pourtorabehsan/sentinel/internal/instance"
	"github.com/stretchr/testify/require"
)

func TestEndpointString(t *testing.T) {
	e := instance.Endpoint{
		Host: "localhost",
		Port: 3306,
	}

	want := "localhost:3306"
	require.Equal(t, want, e.String())
}
