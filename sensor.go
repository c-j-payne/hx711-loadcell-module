package hx711_loadcell

import (
	"context"
	"fmt"
	"sync"

	"github.com/erh/vmodutils"
	"go.viam.com/rdk/components/sensor"
	"go.viam.com/rdk/logging"
	"go.viam.com/rdk/resource"
	rdkutils "go.viam.com/rdk/utils"
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
		name:             config.ResourceName(),
		hx:               hx,
		samples:          samples,
		logger:           logger,
		calibrationSlope: conf.CalibrationSlope,
	}

	if conf.CalibrationSlope != nil {
		if conf.CalibrationOffset != nil {
			s.offset = *conf.CalibrationOffset
		} else {
			if err := s.tare(15); err != nil {
				return nil, fmt.Errorf("tare failed: %w", err)
			}
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

	readLock sync.Mutex

	offset           float64
	calibrationSlope *float64
}

func (s *hx711Sensor) Name() resource.Name {
	return s.name
}

func (s *hx711Sensor) tare(n int) error {
	s.readLock.Lock()
	defer s.readLock.Unlock()

	avg, err := s.hx.ReadAverage(n)
	if err != nil {
		return fmt.Errorf("tare failed: %w", err)
	}
	s.offset = avg
	s.logger.Infof("HX711 tare value: %f", avg)
	return nil
}

func (s *hx711Sensor) getValue() (float64, error) {
	s.readLock.Lock()
	defer s.readLock.Unlock()

	median, err := s.hx.ReadAverage(s.samples)
	if err != nil {
		return 0, err
	}
	result := median - s.offset
	s.logger.Debugf("HX711 getValue: median=%f offset=%f result=%f", median, s.offset, result)
	return result, nil
}

func (s *hx711Sensor) Readings(ctx context.Context, extra map[string]interface{}) (map[string]interface{}, error) {
	rawValue, err := s.getValue()
	if err != nil {
		return nil, err
	}

	readings := map[string]interface{}{
		"raw_value": rawValue,
	}

	if s.calibrationSlope != nil {
		weightKg := (*s.calibrationSlope * rawValue)
		forceN := weightKg * 9.81

		readings["weight_kg"] = weightKg
		readings["force_N"] = forceN
	}

	return readings, nil
}

func (s *hx711Sensor) DoCommand(ctx context.Context, cmd map[string]interface{}) (map[string]interface{}, error) {
	if _, ok := cmd["tare"]; ok {
		if err := s.tare(15); err != nil {
			return nil, err
		}
		return map[string]interface{}{
			"offset": s.offset,
		}, nil
	}

	if _, ok := cmd["get_calibration"]; ok {
		result := map[string]interface{}{
			"offset": s.offset,
		}
		if s.calibrationSlope != nil {
			result["calibration_slope"] = *s.calibrationSlope
		}
		return result, nil
	}

	if weightKg, ok := cmd["calibrate_kg"].(float64); ok {
		rawValue, err := s.getValue()
		if err != nil {
			return nil, err
		}
		slope := weightKg / rawValue
		s.calibrationSlope = &slope

		err = vmodutils.UpdateComponentCloudAttributesFromModuleEnv(
			ctx,
			s.name,
			rdkutils.AttributeMap{"calibration_slope": slope},
			s.logger,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to save calibration to config: %w", err)
		}

		return map[string]interface{}{
			"slope": slope,
		}, nil
	}

	return nil, fmt.Errorf("unknown command, supported commands: tare, get_calibration")
}

func (s *hx711Sensor) Close(ctx context.Context) error {
	if s.hx != nil {
		s.hx.Close()
	}
	return nil
}
