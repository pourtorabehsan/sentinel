package instance_test

import (
	"testing"

	"github.com/pourtorabehsan/sentinel/internal/instance"
	"github.com/stretchr/testify/require"
)

func TestBinlogCoordinatesString(t *testing.T) {
	bc := instance.BinlogCoordinates{
		Filename: "mysql-bin.000001",
		Position: 123,
	}

	want := "mysql-bin.000001:123"
	require.Equal(t, want, bc.String())
}
