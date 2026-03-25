package main

import (
	"go.viam.com/rdk/components/sensor"
	"go.viam.com/rdk/module"
	"go.viam.com/rdk/resource"

	hx711_loadcell "github.com/c_j_payne/hx711_loadcell"
)

func main() {
	module.ModularMain(
		resource.APIModel{sensor.API, hx711_loadcell.SensorModel},
	)
}
