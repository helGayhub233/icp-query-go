package logutil

import (
	"sync"
	"time"
)

// LogEntry represents a single log record.
type LogEntry struct {
	Time    string `json:"time"`
	Message string `json:"message"`
	Level   string `json:"level"`
}

// Collector is a ring buffer for recent log entries.
type Collector struct {
	mu      sync.Mutex
	entries []LogEntry
	maxLen  int
	pos     int
	full    bool
}

// NewCollector creates a log collector with the given capacity.
func NewCollector(maxLen int) *Collector {
	return &Collector{
		entries: make([]LogEntry, maxLen),
		maxLen:  maxLen,
	}
}

// Add appends a log entry.
func (c *Collector) Add(message, level string) {
	c.mu.Lock()
	defer c.mu.Unlock()

	entry := LogEntry{
		Time:    time.Now().Format("2006-01-02 15:04:05"),
		Message: message,
		Level:   level,
	}
	c.entries[c.pos] = entry
	c.pos++
	if c.pos >= c.maxLen {
		c.pos = 0
		c.full = true
	}
}

// Get returns the last n log entries.
func (c *Collector) Get(n int) []LogEntry {
	c.mu.Lock()
	defer c.mu.Unlock()

	total := c.len()
	if n > total {
		n = total
	}
	if n <= 0 {
		return nil
	}

	result := make([]LogEntry, n)
	if c.full {
		start := (c.pos - n + c.maxLen) % c.maxLen
		for i := 0; i < n; i++ {
			result[i] = c.entries[(start+i)%c.maxLen]
		}
	} else {
		start := c.pos - n
		copy(result, c.entries[start:c.pos])
	}
	return result
}

// Clear removes all entries.
func (c *Collector) Clear() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.entries = make([]LogEntry, c.maxLen)
	c.pos = 0
	c.full = false
}

func (c *Collector) len() int {
	if c.full {
		return c.maxLen
	}
	return c.pos
}
