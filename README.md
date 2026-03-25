# HX711 Load Cell Sensor Module

A Viam module for HX711 load cell amplifiers, providing calibrated force and weight measurements.

## Features

- ✅ Raw ADC readings from HX711
- ✅ Calibrated weight measurements (kg)
- ✅ Calibrated force measurements (N)
- ✅ Configurable sampling rates
- ✅ Multi-sensor support
- ✅ Auto-taring for uncalibrated sensors

## Configuration

### Basic Configuration
```json
{
  "data_pin": 5,
  "clock_pin": 6,
  "samples": 10
}
```

### With Calibration
```json
{
  "data_pin": 5,
  "clock_pin": 6,
  "samples": 10,
  "calibration_slope": 0.000123,
  "calibration_offset": -0.5
}
```

## Returns

- `raw_value`: Raw ADC reading
- `weight_kg`: Calibrated weight in kilograms (if calibrated)
- `force_N`: Calibrated force in Newtons (if calibrated)

## DoCommands

### tare
Zeros the sensor by setting the current reading as the offset. Run with no load on the cell.

```python
await loadcell.do_command({"tare": True})
# Returns: {"offset": 123456.0}
```

### calibrate_kg
Calibrates the sensor with a known weight. Place the known weight on the cell, then call with the weight in kg. Saves the computed `calibration_slope` back to the cloud config.

```python
await loadcell.do_command({"calibrate_kg": 5.0})
# Returns: {"slope": 0.000045}
```

### get_calibration
Returns the current calibration values.

```python
await loadcell.do_command({"get_calibration": True})
# Returns: {"offset": 123456.0, "calibration_slope": 0.000045}
```

## Example
```python
from viam.components.sensor import Sensor

loadcell = Sensor.from_robot(robot, "loadcell1")
readings = await loadcell.get_readings()
force = readings["force_N"]
```
