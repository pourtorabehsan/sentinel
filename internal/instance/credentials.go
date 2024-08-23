package instance

import "fmt"

type Credentials struct {
	Username string
	Password string
}

func (c Credentials) String() string {
	return fmt.Sprintf("%s:****", c.Username)
}

func (c Credentials) AuthString() string {
	return fmt.Sprintf("%s:%s", c.Username, c.Password)
}
