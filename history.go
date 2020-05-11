package main

import (
	"math"
	"sort"
	"time"
)

// CacheValue records a particular sample value
type CacheValue struct {
	seq   int
	value float64
}

// History holds a set of CacheValues
type History struct {
	data []float64
	ttl  int
	sz   int
	avg  CacheValue
	med  CacheValue
	vrc  CacheValue
}

// NewHistory creates a history object
func NewHistory(sz int) *History {
	return &History{
		data: make([]float64, sz),
		ttl:  0,
		sz:   sz,
		avg:  CacheValue{seq: -1},
		med:  CacheValue{seq: -1},
		vrc:  CacheValue{seq: -1},
	}
}

// Round rounds a float to given level of precision
func Round(val float64, roundOn float64, places int) float64 {
	var round float64
	pow := math.Pow(10, float64(places))
	digit := pow * val
	_, div := math.Modf(digit)
	if div >= roundOn {
		round = math.Ceil(digit)
	} else {
		round = math.Floor(digit)
	}
	return round / pow
}

// Push adds a value to the history
func (h *History) Push(f float64) {
	h.data[h.ttl%h.sz] = f
	h.ttl++
}

// PushDuration adds a value to the history
func (h *History) PushDuration(d time.Duration) {
	h.Push(float64(d))
}

// Len returns the size of the History
func (h *History) Len() int {
	if h.ttl < h.sz {
		return h.ttl
	}
	return h.sz
}

// Average returns the average value stored in the History
func (h *History) Average() float64 {
	if h.ttl == h.avg.seq {
		return h.avg.value
	}
	total := 0.0
	if h.Len() == 0 {
		return total
	}
	for _, element := range h.data {
		total += element
	}
	h.avg.value = total / float64(h.Len())
	h.avg.seq = h.ttl
	return h.avg.value
}

// Median returns the median value of the history
func (h *History) Median() float64 {
	if h.ttl == h.med.seq {
		return h.med.value
	}
	if h.Len() < 2 {
		h.med.value = h.Average()
	} else {
		data := []float64(h.data[:h.Len()])
		sort.Float64s(data)
		h.med.value = data[h.Len()/2]
	}
	h.med.seq = h.ttl
	return h.med.value
}

// Variance computes and returns the variance for the data in the History
func (h *History) Variance() float64 {
	if h.ttl == h.vrc.seq {
		return h.vrc.value
	}
	variance := 0.0
	if h.Len() < 1 {
		return variance
	}
	avg := h.Average()
	for _, n := range h.data[:h.Len()] {
		variance = variance + math.Pow(avg-n, 2.0)
	}
	h.vrc.value = variance / float64(h.Len())
	h.vrc.seq = h.ttl
	return h.vrc.value
}

// Stddev returns the standard deviation of the history
func (h *History) Stddev() float64 {
	return math.Sqrt(h.Variance())
}
