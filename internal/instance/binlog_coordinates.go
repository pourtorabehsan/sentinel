package instance

import "fmt"

type BinlogCoordinates struct {
	Filename string
	Position int
}

func (c BinlogCoordinates) String() string {
	return fmt.Sprintf("%s:%d", c.Filename, c.Position)
}
