package main

import (
	"testing"
)

func TestHistory(t *testing.T) {
	sizes := [...]int{11, 20, 50, 100, 200}
	list := []float64{71.0, 36.3, 54.3, 52.3, 56.2, 39.1, 14.6, 56.7,
		95.0, 5.3, 13.0, 33.7, 1.4, 14.4, 88.2, 16.0, 57.2, 73.5, 10.5,
		70.2, 64.3, 73.3, 14.2, 44.4, 14.2, 72.6, 29.5, 52.5, 72.5,
		39.5, 56.1, 13.4, 74.2, 85.0, 61.2, 12.4, 52.0, 12.0, 1.5, 49.8,
		21.5, 94.4, 58.9, 18.3, 98.0, 43.4, 62.1, 81.9, 71.7, 68.8,
		66.1, 79.9, 0.1, 87.2, 68.3, 81.8, 96.6, 19.4, 95.1, 27.5, 8.8,
		77.3, 82.1, 81.6, 61.2, 28.3, 25.7, 2.7, 74.3, 5.0, 68.9, 46.7,
		9.0, 62.2, 44.6, 26.2, 14.6, 86.1, 33.4, 1.4, 33.1, 21.4, 28.5,
		96.3, 41.0, 33.4, 56.5, 84.3, 37.3, 97.0, 40.0, 43.8, 88.3,
		13.3, 14.1, 50.6, 54.5, 43.8, 33.2, 50.4}
	var h History

	for _, sz := range sizes {
		h = *NewHistory(sz)
		if h.sz != sz {
			t.Errorf("Expected size=%d, found %d")
		}
		for _, f := range list {
			h.Push(f)
		}
	}

	t.Run("Average", func(t *testing.T) {
		avg := h.Average()
		if int32(avg*10.0) != 479 {
			t.Errorf("Average was %0.1f, expected 47.9", avg)
		}
	})
	t.Run("Median", func(t *testing.T) {
		med := h.Median()
		if int32(med*10.0) != 504 {
			t.Errorf("Median was %0.1f, expected 50.4", med)
		}
	})
	t.Run("Variance", func(t *testing.T) {
		variance := h.Variance()
		if int32(variance*10.0) != 8054 {
			t.Errorf("Variance was %0.1f, expected 805.4", variance)
		}
		v2 := h.Variance()
		if variance != v2 {
			t.Errorf("Cached value should have been presented")
		}
	})
	t.Run("Empty History OK", func(t *testing.T) {
		hs := NewHistory(10)
		if hs.Average() != 0.0 {
			t.Errorf("Average was %0.1f, expected 0.0", hs.Average())
		}
		if hs.Median() != 0.0 {
			t.Errorf("Median was %0.1f, expected 0.0", hs.Median())
		}
		if hs.Variance() != 0.0 {
			t.Errorf("Variance was %0.1f, expected 0.0", hs.Variance())
		}
	})
}
