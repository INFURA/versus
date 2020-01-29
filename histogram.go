package main

import "sort"

// TODO: Replace histogram implementation with a sparse bucket based one so
// it's memory-bounded.

type histogram struct {
	all   []float64
	min   float64
	max   float64
	total float64
}

func (h *histogram) Add(point float64) {
	h.all = append(h.all, point)
	h.total += point
	if len(h.all) == 1 || h.min > point {
		h.min = point
	}
	if h.max < point {
		h.max = point
	}
}

func (h *histogram) Total() float64 {
	return h.total
}

func (h *histogram) Min() float64 {
	return h.min
}

func (h *histogram) Max() float64 {
	return h.max
}

func (h *histogram) Average() float64 {
	return h.total / float64(len(h.all))
}

func (h *histogram) Variance() float64 {
	// Population variance
	mean := h.Average()
	sum := 0.0
	for _, v := range h.all {
		delta := v - mean
		sum += delta * delta
	}
	return sum / float64(h.Len())
}

func (h *histogram) Len() int {
	return len(h.all)
}

// Percentiles takes buckets in whole percentages (e.g. 95 is 95%) and returns
// a slice with percentile values in the corresponding index. Buckets must be
// in-order.
func (h *histogram) Percentiles(buckets ...int) []float64 {
	r := make([]float64, len(buckets))
	if len(h.all) == 0 {
		return r
	}

	sort.Float64s(h.all)

	for i, j := 0, 0; i < len(h.all) || j < len(buckets); i++ {
		if i >= len(h.all) {
			// Fill up the remaining buckets with the highest value.
			r[j] = h.all[len(h.all)-1]
			j++
			continue
		}

		current := i * 100 / len(h.all)
		if current >= buckets[j] {
			r[j] = h.all[i]
			j++
		}

		if j >= len(buckets) {
			break
		}
	}
	return r
}
