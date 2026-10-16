package cache_test

import (
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/Soviann/trackarr/internal/cache"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCache_Nominal(t *testing.T) {
	c := cache.New(10, 5*time.Minute)

	// Set and Get hit
	c.Set("k1", "val1", cache.TagTitles)
	c.Set("k2", 42, cache.TagStats)
	c.Set("k3", []string{"a", "b"}, cache.TagCalendar, cache.TagTitles)

	v1, ok1 := c.Get("k1")
	assert.True(t, ok1)
	assert.Equal(t, "val1", v1)

	v2, ok2 := c.Get("k2")
	assert.True(t, ok2)
	assert.Equal(t, 42, v2)

	v3, ok3 := c.Get("k3")
	assert.True(t, ok3)
	assert.Equal(t, []string{"a", "b"}, v3)

	assert.Equal(t, 3, c.Len())

	// Invalidate TagTitles should evict k1 and k3, keeping k2
	c.Invalidate(cache.TagTitles)

	_, ok1After := c.Get("k1")
	assert.False(t, ok1After)

	v2After, ok2After := c.Get("k2")
	assert.True(t, ok2After)
	assert.Equal(t, 42, v2After)

	_, ok3After := c.Get("k3")
	assert.False(t, ok3After)

	assert.Equal(t, 1, c.Len())

	// Clear should remove all remaining items
	c.Clear()
	assert.Equal(t, 0, c.Len())
	_, ok2Cleared := c.Get("k2")
	assert.False(t, ok2Cleared)
}

func TestCache_Boundary(t *testing.T) {
	t.Run("maxEntries eviction boundary", func(t *testing.T) {
		// Capacity = 3
		c := cache.New(3, 5*time.Minute)

		c.Set("k1", "v1", cache.TagTitles)
		c.Set("k2", "v2", cache.TagTitles)
		c.Set("k3", "v3", cache.TagTitles)
		assert.Equal(t, 3, c.Len())

		// Adding 4th item should evict oldest (k1)
		c.Set("k4", "v4", cache.TagTitles)
		assert.Equal(t, 3, c.Len())

		_, ok1 := c.Get("k1")
		assert.False(t, ok1, "oldest item k1 should be evicted")

		_, ok2 := c.Get("k2")
		assert.True(t, ok2)
		_, ok3 := c.Get("k3")
		assert.True(t, ok3)
		_, ok4 := c.Get("k4")
		assert.True(t, ok4)

		// Overwrite existing key should not increase length
		c.Set("k4", "v4_updated", cache.TagTitles)
		assert.Equal(t, 3, c.Len())
		v4Up, _ := c.Get("k4")
		assert.Equal(t, "v4_updated", v4Up)
	})

	t.Run("TTL expiration boundary", func(t *testing.T) {
		c := cache.New(10, 50*time.Millisecond)

		c.SetWithTTL("fast_exp", "val", 25*time.Millisecond, cache.TagTitles)
		c.SetWithTTL("slow_exp", "val", 500*time.Millisecond, cache.TagStats)

		// Immediately available
		v, ok := c.Get("fast_exp")
		assert.True(t, ok)
		assert.Equal(t, "val", v)

		// Wait for fast_exp to expire
		time.Sleep(35 * time.Millisecond)

		_, okExpired := c.Get("fast_exp")
		assert.False(t, okExpired, "fast_exp should be expired")

		// slow_exp still alive
		_, okSlow := c.Get("slow_exp")
		assert.True(t, okSlow, "slow_exp should still be valid")
	})

	t.Run("empty tag list and nil values", func(t *testing.T) {
		c := cache.New(5, 5*time.Minute)

		// Nil value with no tags
		c.Set("nil_key", nil)
		v, ok := c.Get("nil_key")
		assert.True(t, ok)
		assert.Nil(t, v)

		// Invalidation on empty tag list or non-existent tag is no-op
		c.Invalidate()
		c.Invalidate("non_existent_tag")
		assert.Equal(t, 1, c.Len())

		// Fallback for non-positive constructor params
		fallbackCache := cache.New(0, 0)
		assert.NotNil(t, fallbackCache)
		fallbackCache.Set("k", "v")
		fVal, fOk := fallbackCache.Get("k")
		assert.True(t, fOk)
		assert.Equal(t, "v", fVal)
	})
}

func TestCache_Concurrency(t *testing.T) {
	c := cache.New(50, 200*time.Millisecond)
	var wg sync.WaitGroup

	numGoroutines := 20
	numOps := 50

	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			for j := 0; j < numOps; j++ {
				key := fmt.Sprintf("key_%d", (id+j)%30)
				tag := cache.TagTitles
				if j%2 == 0 {
					tag = cache.TagStats
				} else if j%3 == 0 {
					tag = cache.TagCalendar
				}

				switch j % 4 {
				case 0:
					c.Set(key, j, tag)
				case 1:
					c.Get(key)
				case 2:
					if j%10 == 0 {
						c.Invalidate(tag)
					}
				case 3:
					_ = c.Len()
					_ = c.Keys()
				}
			}
		}(i)
	}

	wg.Wait()
	require.True(t, c.Len() <= 50)
}
