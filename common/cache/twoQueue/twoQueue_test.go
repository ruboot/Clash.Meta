package twoQ

import (
	"github.com/stretchr/testify/assert"
	"testing"
)

var entries = []struct {
	key   string
	value string
}{
	{"1", "one"},
	{"2", "two"},
	{"3", "three"},
	{"4", "four"},
	{"5", "five"},
}

func TestTwoQCache(t *testing.T) {

	size := 4
	c, _ := New[string, string](size)

	for _, e := range entries {
		c.Set(e.key, e.value)
	}

	// 访问部分key使其成为热点数据
	c.Get("1")
	c.Get("2")
	c.Get("2")

	// 验证cold list中的数据
	value, ok := c.Get("5")
	assert.True(t, ok)
	assert.Equal(t, "five", value)

	// 插入新项时删除cold list中元素
	c.Set("6", "six")
	_, ok = c.Get("3")
	assert.False(t, ok)

	// 验证热点key未被删除
	_, ok = c.Get("2")
	assert.True(t, ok)

	for _, e := range entries {
		c.Delete(e.key)

		_, ok := c.Get(e.key)
		assert.False(t, ok)
	}
}
