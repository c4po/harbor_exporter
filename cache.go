package main

import (
	"github.com/prometheus/client_golang/prometheus"
	"sync"
	"time"
)

type Cache struct {
	cacheEnabled    bool
	cacheDuration   time.Duration
	lastCollectTime time.Time
	cache           []prometheus.Metric
	collectMutex    sync.Mutex
}

func NewCache(cacheEnabled bool, cacheDuration time.Duration) *Cache {
	return &Cache{
		cacheEnabled:    cacheEnabled,
		cacheDuration:   cacheDuration,
		cache:           make([]prometheus.Metric, 0),
		lastCollectTime: time.Unix(0, 0),
		collectMutex:    sync.Mutex{},
	}
}

func (c *Cache) ReplayMetrics(outCh chan<- prometheus.Metric) bool {
	if c.cacheEnabled {
		c.collectMutex.Lock()
		defer c.collectMutex.Unlock()
		expiry := c.lastCollectTime.Add(c.cacheDuration)
		if time.Now().Before(expiry) {
			// Return cached
			for _, cachedMetric := range c.cache {
				outCh <- cachedMetric
			}
			return true
		}
		// Reset cache for fresh sampling, but re-use underlying array
		c.cache = c.cache[:0]
	}
	return false
}

func (c *Cache) StoreAndForwaredMetrics(outCh chan<- prometheus.Metric) (chan prometheus.Metric, *sync.WaitGroup) {
	samplesCh := make(chan prometheus.Metric)
	// Use WaitGroup to ensure outCh isn't closed before the goroutine is finished
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		if c.cacheEnabled {
			c.collectMutex.Lock()
			defer c.collectMutex.Unlock()
			// Prometheus will reject duplicate metrics
			c.cache = c.cache[:0]
		}
		for metric := range samplesCh {
			outCh <- metric
			if c.cacheEnabled {
				c.cache = append(c.cache, metric)
			}
		}
		wg.Done()
		c.lastCollectTime = time.Now()
	}()
	return samplesCh, &wg
}
