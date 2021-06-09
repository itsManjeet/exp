// Copyright 2021 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Cache is a simple cache.
// It is an example of using event metrics.
package cache

import (
	"context"

	"golang.org/x/exp/event"
	"golang.org/x/exp/event/keys"
)

type Cache struct {
	m      map[string]interface{}
	getter func(string) interface{}
	count  *event.Counter
}

func New(getter func(string) interface{}, metricNamespace string) *Cache {
	c := &Cache{
		m:      map[string]interface{}{},
		getter: getter,
		count:  event.NewCounter("cacheProbes"),
	}
	if metricNamespace != "" {
		c.count.SetNamespace(metricNamespace)
	}
	return c
}

var hit = keys.Bool("hit")

func (c *Cache) Get(ctx context.Context, key string) interface{} {
	v, ok := c.m[key]
	if ok {
		c.count.To(ctx).With(hit.Of(true)).Record(1)
		return v
	}
	v = c.getter(key)
	c.m[key] = v
	c.count.To(ctx).With(hit.Of(false)).Record(1)
	return v
}
