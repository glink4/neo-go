package manifest

import (
	"bytes"
	"encoding/json"

	"github.com/nspcc-dev/neo-go/pkg/util"
)

// ContainerType represents container type.
type ContainerType byte

const (
	// Start from 1 to catch errors where container type is not initialized.
	_ ContainerType = iota
	// StringContainer contains string items.
	StringContainer
	// Uint160Container contains util.Uint160 items.
	Uint160Container
)

const invalidContainerType = "invalid container type"

// Container is a generic container which can be wildcard.
// Value is nil iff it contains all possible values.
type Container struct {
	Type  ContainerType
	Value interface{}
}

// NewContainer returns new wildcard container of the specified type.
func NewContainer(t ContainerType) *Container {
	return &Container{
		Type: t,
	}
}

// Strings casts value to a slice of strings.
func (c *Container) Strings() []string {
	if c.Value == nil {
		return nil
	}
	return c.Value.([]string)
}

// Contains checks if value is in the container.
func (c *Container) Contains(v interface{}) bool {
	switch c.Type {
	case StringContainer:
		val := v.(string)
		if c.IsWildcard() {
			return true
		}
		for _, s := range c.Strings() {
			if val == s {
				return true
			}
		}
	case Uint160Container:
		val := v.(util.Uint160)
		if c.IsWildcard() {
			return true
		}
		for _, u := range c.Uint160s() {
			if u.Equals(val) {
				return true
			}
		}
	default:
		panic(invalidContainerType)
	}
	return false
}

// Uint160s casts value to a slice of util.Uint160.
func (c *Container) Uint160s() []util.Uint160 {
	if c.Value == nil {
		return nil
	}
	return c.Value.([]util.Uint160)
}

// IsWildcard returns true iff container is wildcard i.e.
// contains every possible value.
func (c *Container) IsWildcard() bool {
	return c.Value == nil
}

// Restrict transforms container into an empty one.
func (c *Container) Restrict() {
	switch c.Type {
	case StringContainer:
		c.Value = []string{}
	case Uint160Container:
		c.Value = []util.Uint160{}
	default:
		panic(invalidContainerType)
	}
}

// Add adds v to the container.
func (c *Container) Add(v interface{}) {
	switch c.Type {
	case StringContainer:
		s := v.(string)
		if c.Value == nil {
			c.Value = []string{s}
		} else {
			c.Value = append(c.Value.([]string), s)
		}
	case Uint160Container:
		u := v.(util.Uint160)
		if c.Value == nil {
			c.Value = []util.Uint160{u}
		} else {
			c.Value = append(c.Value.([]util.Uint160), u)
		}
	default:
		panic(invalidContainerType)
	}
}

// MarshalJSON implements json.Marshaler interface.
func (c *Container) MarshalJSON() ([]byte, error) {
	if c.IsWildcard() {
		return []byte(`"*"`), nil
	}
	return json.Marshal(c.Value)
}

// UnmarshalJSON implements json.Unmarshaler interface.
func (c *Container) UnmarshalJSON(data []byte) error {
	if bytes.Equal(data, []byte(`"*"`)) {
		c.Value = nil
		return nil
	}
	switch c.Type {
	case StringContainer:
		ss := []string{}
		if err := json.Unmarshal(data, &ss); err != nil {
			return err
		}
		c.Value = ss
	case Uint160Container:
		us := []util.Uint160{}
		if err := json.Unmarshal(data, &us); err != nil {
			return err
		}
		c.Value = us
	default:
		// we panic here because container type
		// is expected to be set before unmarhaling.
		panic(invalidContainerType)
	}
	return nil
}
