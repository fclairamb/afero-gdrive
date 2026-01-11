package cache

import (
	"fmt"
	"testing"

	ast "github.com/stretchr/testify/assert"
)

func TestBasics(t *testing.T) {
	c := NewCache()
	assert := ast.New(t)

	c.Set("key1", "value1")
	c.Set("key2", "value2")

	v, ok := c.Get("key1")
	assert.Equal("value1", v)
	assert.True(ok)

	assert.Nil(c.GetValue("key4"))

	c.Delete("key1")

	v, ok = c.Get("key1")
	assert.Nil(v)
	assert.False(ok)

	assert.Equal("value2", c.GetValue("key2"))

	c.CleanupEverything()

	assert.Nil(c.GetValue("key2"))
}

func TestPrefixCleanup(t *testing.T) {
	c := NewCache()
	assert := ast.New(t)

	c.Set("pre1-key1", "value1")
	c.Set("pre1-key2", "value2")
	c.Set("pre2-key1", "value3")
	c.Set("pre2-key2", "value4")

	assert.Equal(2, c.CleanupByPrefix("pre1"))
	assert.Nil(c.GetValue("pre1-key1"))
	assert.Equal("value3", c.GetValue("pre2-key1"))
}

func BenchmarkGet(b *testing.B) {
	c := NewCache()
	nbKeys := 100

	for i := 0; i < nbKeys; i++ {
		c.Set(fmt.Sprintf("key %d", i), fmt.Sprintf("value %d", i))
	}

	keyName := fmt.Sprintf("key %d", nbKeys/2)

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			c.GetValue(keyName)
		}
	})
}

func BenchmarkSet(b *testing.B) {
	c := NewCache()
	nbKeys := 100

	for i := 0; i < nbKeys; i++ {
		c.Set(fmt.Sprintf("key %d", i), fmt.Sprintf("value %d", i))
	}

	keyName := fmt.Sprintf("key %d", nbKeys/2)
	value := fmt.Sprintf("value %d", nbKeys/2)

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			c.Set(keyName, value)
		}
	})
}
