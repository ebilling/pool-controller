package main

import (
	"math"
	"sort"
	"time"
)

type CacheValue struct {
	seq   int
	value float64
}

type History struct {
	data []float64
	ttl  int
	sz   int
	avg  CacheValue
	med  CacheValue
	vrc  CacheValue
}

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

func (h *History) Push(f float64) {
	h.data[h.ttl%h.sz] = f
	h.ttl++
}

func (h *History) PushDuration(d time.Duration) {
	h.Push(float64(d))
}

func (h *History) Len() int {
	if h.ttl < h.sz {
		return h.ttl
	}
	return h.sz
}

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

func (h *History) Stddev() float64 {
	return math.Sqrt(h.Variance())
}
