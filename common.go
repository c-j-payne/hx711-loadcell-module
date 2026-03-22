package hx711_loadcell

import (
	"sort"

	"go.viam.com/rdk/resource"
)

var family = resource.ModelNamespace("chris").WithFamily("sensor")

func average(values []float64) float64 {
	n := len(values)

	sort.Float64s(values)

	// Trim 20% from each end
	trim := int(float64(n) * 0.2)
	trimmed := values[trim : n-trim]

	var sum float64
	for _, v := range trimmed {
		sum += v
	}
	return sum / float64(len(trimmed))
}
