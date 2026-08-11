package fixture

import "testing"

type Fixture struct {
	t *testing.T
}

func New(t *testing.T) *Fixture {
	return &Fixture{t: t}
}
