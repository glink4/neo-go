package manifest

import (
	"encoding/json"
	"testing"

	"github.com/nspcc-dev/neo-go/pkg/internal/random"
	"github.com/nspcc-dev/neo-go/pkg/util"
	"github.com/stretchr/testify/require"
)

func TestContainer_Restrict(t *testing.T) {
	t.Run("string", func(t *testing.T) {
		c := NewContainer(StringContainer)
		require.True(t, c.IsWildcard())
		require.True(t, c.Contains("abc"))
		c.Restrict()
		require.False(t, c.IsWildcard())
		require.False(t, c.Contains("abc"))
		require.Equal(t, 0, len(c.Strings()))
	})

	t.Run("uint160", func(t *testing.T) {
		c := NewContainer(Uint160Container)
		u := random.Uint160()
		require.True(t, c.IsWildcard())
		require.True(t, c.Contains(u))
		c.Restrict()
		require.False(t, c.IsWildcard())
		require.False(t, c.Contains(u))
		require.Equal(t, 0, len(c.Uint160s()))
	})
}

func TestContainer_Add(t *testing.T) {
	t.Run("string", func(t *testing.T) {
		c := NewContainer(StringContainer)
		require.Equal(t, []string(nil), c.Strings())

		c.Add("abc")
		require.True(t, c.Contains("abc"))
		require.False(t, c.Contains("aaa"))

		require.Panics(t, func() { c.Add(random.Uint160()) })
	})

	t.Run("uint160", func(t *testing.T) {
		c := NewContainer(Uint160Container)
		require.Equal(t, []util.Uint160(nil), c.Uint160s())

		exp := []util.Uint160{random.Uint160(), random.Uint160()}
		for i := range exp {
			c.Add(exp[i])
		}
		for i := range exp {
			require.True(t, c.Contains(exp[i]))
		}
		require.False(t, c.Contains(random.Uint160()))

		require.Panics(t, func() { c.Add("abc") })
	})
}

func TestContainer_Invalid(t *testing.T) {
	c := NewContainer(ContainerType(0xFF))
	require.Panics(t, func() { c.Restrict() })
	require.Panics(t, func() { c.Add("abc") })

	sc := NewContainer(StringContainer)
	sc.Add("abc")
	data, err := sc.MarshalJSON()
	require.NoError(t, err)
	require.Panics(t, func() { _ = c.UnmarshalJSON(data) })
	require.Panics(t, func() { c.Restrict() })
	require.Panics(t, func() { c.Contains("") })
}

func TestContainer_MarshalJSON(t *testing.T) {
	t.Run("string", func(t *testing.T) {
		t.Run("wildcard", func(t *testing.T) {
			expected := NewContainer(StringContainer)
			testMarshalUnmarshal(t, expected, NewContainer(StringContainer))
		})

		t.Run("empty", func(t *testing.T) {
			expected := NewContainer(StringContainer)
			expected.Restrict()
			testMarshalUnmarshal(t, expected, NewContainer(StringContainer))
		})

		t.Run("non-empty", func(t *testing.T) {
			expected := NewContainer(StringContainer)
			expected.Add("string1")
			expected.Add("string2")
			testMarshalUnmarshal(t, expected, NewContainer(StringContainer))
		})

		t.Run("invalid", func(t *testing.T) {
			js := []byte(`[123]`)
			c := NewContainer(StringContainer)
			require.Error(t, json.Unmarshal(js, c))
		})
	})

	t.Run("uint160", func(t *testing.T) {
		t.Run("wildcard", func(t *testing.T) {
			expected := NewContainer(Uint160Container)
			testMarshalUnmarshal(t, expected, NewContainer(Uint160Container))
		})

		t.Run("empty", func(t *testing.T) {
			expected := NewContainer(Uint160Container)
			expected.Restrict()
			testMarshalUnmarshal(t, expected, NewContainer(Uint160Container))
		})

		t.Run("non-empty", func(t *testing.T) {
			expected := NewContainer(Uint160Container)
			expected.Add(random.Uint160())
			testMarshalUnmarshal(t, expected, NewContainer(Uint160Container))
		})

		t.Run("invalid", func(t *testing.T) {
			js := []byte(`["notahex"]`)
			c := NewContainer(Uint160Container)
			require.Error(t, json.Unmarshal(js, c))
		})
	})
}
