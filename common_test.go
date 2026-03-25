package hx711_loadcell

import (
	"testing"

	"go.viam.com/test"
)

func TestAverage(t *testing.T) {
	test.That(t, average([]float64{5.1}), test.ShouldEqual, 5.1)
	test.That(t, average([]float64{1, 5}), test.ShouldEqual, 3)
	test.That(t, average([]float64{1, 1, 4}), test.ShouldEqual, 2)
	test.That(t, average([]float64{1, 2, 2, 3}), test.ShouldEqual, 2)
	test.That(t, average([]float64{1, 1, 1, 1, 6}), test.ShouldEqual, 1)

}
