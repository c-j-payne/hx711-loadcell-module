import asyncio
from typing import Any, ClassVar, Mapping, Optional, Sequence
from viam.components.sensor import Sensor
from viam.module.module import Module
from viam.proto.app.robot import ComponentConfig
from viam.proto.common import ResourceName
from viam.resource.base import ResourceBase
from viam.resource.types import Model, ModelFamily
from viam.utils import SensorReading

# Import HX711 from local directory or installed package
try:
    from hx711 import HX711
except ImportError:
    # If bundled in the module
    import sys
    import os
    sys.path.insert(0, os.path.dirname(__file__))
    from hx711 import HX711

class HX711Sensor(Sensor):
    """HX711 Load Cell Sensor with Calibration Support"""
    MODEL: ClassVar[Model] = Model(ModelFamily("chris", "sensor"), "hx711")
    
    def __init__(self, name: str):
        super().__init__(name)
        self.hx = None
        self.calibration_slope = None
        self.calibration_offset = None
        self.output_unit = "raw"  # default to raw readings
    
    @classmethod
    def new(cls, config: ComponentConfig, dependencies: Mapping[ResourceName, ResourceBase]):
        sensor = cls(config.name)
        sensor.reconfigure(config, dependencies)
        return sensor
    
    @classmethod
    def validate_config(cls, config: ComponentConfig) -> Sequence[str]:
        return []
    
    def reconfigure(self, config: ComponentConfig, dependencies: Mapping[ResourceName, ResourceBase]):
        data_pin = int(config.attributes.fields["data_pin"].number_value)
        clock_pin = int(config.attributes.fields["clock_pin"].number_value)
        
        # Get samples config safely (default to 1 for speed)
        samples_field = config.attributes.fields.get("samples")
        if samples_field and hasattr(samples_field, 'number_value'):
            self.samples = int(samples_field.number_value)
        else:
            self.samples = 1
        
        # Get calibration parameters (optional)
        slope_field = config.attributes.fields.get("calibration_slope")
        if slope_field and hasattr(slope_field, 'number_value'):
            self.calibration_slope = float(slope_field.number_value)
        
        offset_field = config.attributes.fields.get("calibration_offset")
        if offset_field and hasattr(offset_field, 'number_value'):
            self.calibration_offset = float(offset_field.number_value)
        
        # Get output unit preference (default to "N" if calibrated, "raw" if not)
        unit_field = config.attributes.fields.get("output_unit")
        if unit_field and hasattr(unit_field, 'string_value'):
            self.output_unit = unit_field.string_value
        else:
            # Default to N if calibrated, raw if not
            if self.calibration_slope is not None and self.calibration_offset is not None:
                self.output_unit = "N"
            else:
                self.output_unit = "raw"
        
        if self.hx:
            try:
                self.hx.power_down()
            except:
                pass
        
        self.hx = HX711(data_pin, clock_pin)
        self.hx.reset()
        
        # NEVER tare when calibration is provided - we need raw ADC values
        if self.calibration_slope is None or self.calibration_offset is None:
            self.hx.tare()
    
    async def get_readings(self, *, extra: Optional[Mapping[str, Any]] = None,
                      timeout: Optional[float] = None, **kwargs) -> Mapping[str, SensorReading]:
        try:
            # Get raw ADC reading from HX711 using get_value() - no tare offset
            raw_value = self.hx.get_value(self.samples)
            
            # Build response dictionary
            readings = {"raw_value": round(raw_value, 2)}
            
            # Apply calibration if available
            if self.calibration_slope is not None and self.calibration_offset is not None:
                weight_kg = (self.calibration_slope * raw_value) + self.calibration_offset
                force_n = weight_kg * 9.81  # Convert kg to Newtons
                
                readings["weight_kg"] = round(weight_kg, 4)
                readings["force_N"] = round(force_n, 4)
            
            return readings
            
        except Exception as e:
            return {"error": str(e)}
    
    async def close(self):
        if self.hx:
            self.hx.power_down()

if __name__ == "__main__":
    from viam.resource.registry import Registry, ResourceCreatorRegistration
    
    # FIRST: Register the model in the global registry
    Registry.register_resource_creator(
        Sensor.API,
        HX711Sensor.MODEL,
        ResourceCreatorRegistration(HX711Sensor.new, HX711Sensor.validate_config)
    )
    
    # THEN: Add it to the module and start
    async def main():
        module = Module.from_args()
        module.add_model_from_registry(Sensor.API, HX711Sensor.MODEL)
        await module.start()
    
    asyncio.run(main())
