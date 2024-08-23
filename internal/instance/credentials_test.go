package instance_test

import (
	"testing"

	"github.com/pourtorabehsan/sentinel/internal/instance"
	"github.com/stretchr/testify/require"
)

func TestCredentialsString(t *testing.T) {
	c := instance.Credentials{
		Username: "root",
		Password: "password",
	}

	want := "root:****"
	require.Equal(t, want, c.String())
}

func TestCredentialsAuthString(t *testing.T) {
	c := instance.Credentials{
		Username: "root",
		Password: "password",
	}

	want := "root:password"
	require.Equal(t, want, c.AuthString())
}
