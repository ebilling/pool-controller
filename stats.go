package main

import (
	"math"
	"sort"
)

func Average(list []float64) (float64) {
	total := 0.0
	for _, element := range list {
		total += element
	}
	return total/float64(len(list))
}

func Median(list []float64) (float64) {
	if len(list) < 2 {
		return Average(list)
	}
	sort.Float64s(list)
	return list[len(list)/2]
}

func Variance(list []float64) (float64) {
	if len(list) < 1 {
		return 0.0
	}
	avg := Average(list)
	variance := 0.0
	for _, n := range list {
		variance = variance + math.Pow(avg - n, 2.0)		
	}
	return variance/float64(len(list))
}

func Stddev(list []float64) (float64) {
	return math.Sqrt(Variance(list))
}
