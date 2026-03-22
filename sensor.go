package hx711_loadcell

import (
	"context"
	"fmt"

	"go.viam.com/rdk/components/sensor"
	"go.viam.com/rdk/logging"
	"go.viam.com/rdk/resource"
	"go.viam.com/utils"
)

var SensorModel = family.WithModel("hx711")

type SensorConfig struct {
	DataPin           int      `json:"data_pin"`
	ClockPin          int      `json:"clock_pin"`
	Samples           int      `json:"samples,omitempty"`
	Gain              int      `json:"gain,omitempty"`
	CalibrationSlope  *float64 `json:"calibration_slope,omitempty"`
	CalibrationOffset *float64 `json:"calibration_offset,omitempty"`
}

func (cfg *SensorConfig) Validate(path string) ([]string, []string, error) {
	if cfg.DataPin == 0 {
		return nil, nil, utils.NewConfigValidationFieldRequiredError(path, "data_pin")
	}
	if cfg.ClockPin == 0 {
		return nil, nil, utils.NewConfigValidationFieldRequiredError(path, "clock_pin")
	}
	return nil, nil, nil
}

func init() {
	resource.RegisterComponent(
		sensor.API,
		SensorModel,
		resource.Registration[sensor.Sensor, *SensorConfig]{
			Constructor: newSensor,
		})
}

func newSensor(ctx context.Context, deps resource.Dependencies, config resource.Config, logger logging.Logger) (sensor.Sensor, error) {
	conf, err := resource.NativeConfig[*SensorConfig](config)
	if err != nil {
		return nil, err
	}

	chip, err := findGPIOChip(logger)
	if err != nil {
		return nil, err
	}

	gain := conf.Gain
	if gain == 0 {
		gain = 128
	}

	hx, err := NewHX711(chip, conf.DataPin, conf.ClockPin, gain, logger)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize HX711: %w", err)
	}

	samples := conf.Samples
	if samples <= 0 {
		samples = 1
	}

	s := &hx711Sensor{
		name:              config.ResourceName(),
		hx:                hx,
		samples:           samples,
		logger:            logger,
		calibrationSlope:  conf.CalibrationSlope,
		calibrationOffset: conf.CalibrationOffset,
	}

	// Only tare when no calibration is provided (calibration needs raw ADC values)
	if conf.CalibrationSlope == nil || conf.CalibrationOffset == nil {
		if err := hx.Tare(15); err != nil {
			return nil, fmt.Errorf("tare failed: %w", err)
		}
	}

	return s, nil
}

type hx711Sensor struct {
	resource.AlwaysRebuild

	name    resource.Name
	hx      *HX711
	samples int
	logger  logging.Logger

	calibrationSlope  *float64
	calibrationOffset *float64
}

func (s *hx711Sensor) Name() resource.Name {
	return s.name
}

func (s *hx711Sensor) Readings(ctx context.Context, extra map[string]interface{}) (map[string]interface{}, error) {
	rawValue, err := s.hx.GetValue(s.samples)
	if err != nil {
		return nil, err
	}

	readings := map[string]interface{}{
		"raw_value": roundTo(rawValue, 2),
	}

	if s.calibrationSlope != nil && s.calibrationOffset != nil {
		weightKg := (*s.calibrationSlope * rawValue) + *s.calibrationOffset
		forceN := weightKg * 9.81

		readings["weight_kg"] = roundTo(weightKg, 4)
		readings["force_N"] = roundTo(forceN, 4)
	}

	return readings, nil
}

func (s *hx711Sensor) DoCommand(ctx context.Context, cmd map[string]interface{}) (map[string]interface{}, error) {
	return nil, nil
}

func (s *hx711Sensor) Close(ctx context.Context) error {
	if s.hx != nil {
		s.hx.Close()
	}
	return nil
}
