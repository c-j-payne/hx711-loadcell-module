package hx711_loadcell

import (
	"fmt"
	"runtime"
	"sync"
	"time"

	"github.com/warthog618/gpiod"
	"go.viam.com/rdk/logging"
)

// HX711 drives an HX711 load cell amplifier via GPIO bit-banging
// using the Linux GPIO character device (/dev/gpiochipN).
type HX711 struct {
	mu     sync.Mutex
	dout   *gpiod.Line
	pdSck  *gpiod.Line
	gain   int // extra clock pulses: 1→gain128, 2→gain32, 3→gain64
	logger logging.Logger
}

// findGPIOChip finds a working gpiochip device by trying to open each one.
// Pi 5 uses gpiochip4 for user GPIO, older Pis use gpiochip0.
func findGPIOChip(logger logging.Logger) (string, error) {
	for _, name := range []string{"gpiochip4", "gpiochip0", "gpiochip1", "gpiochip2", "gpiochip3"} {
		chip, err := gpiod.NewChip(name)
		if err != nil {
			logger.Debugf("GPIO chip %s: %v", name, err)
			continue
		}
		chip.Close()
		logger.Infof("Using GPIO chip: %s", name)
		return name, nil
	}
	return "", fmt.Errorf("no working GPIO chip found")
}

func NewHX711(chipName string, doutPin, pdSckPin, gain int, logger logging.Logger) (*HX711, error) {
	h := &HX711{
		logger: logger,
	}

	// Map gain value to number of extra clock pulses
	switch gain {
	case 128:
		h.gain = 1
	case 64:
		h.gain = 3
	case 32:
		h.gain = 2
	default:
		h.gain = 1
	}

	chip, err := gpiod.NewChip(chipName)
	if err != nil {
		return nil, fmt.Errorf("failed to open GPIO chip %q: %w", chipName, err)
	}

	// Request DOUT as input
	dout, err := chip.RequestLine(doutPin, gpiod.AsInput)
	if err != nil {
		chip.Close()
		return nil, fmt.Errorf("failed to request DOUT pin %d: %w", doutPin, err)
	}
	h.dout = dout

	// Request PD_SCK as output, initially low
	pdSck, err := chip.RequestLine(pdSckPin, gpiod.AsOutput(0))
	if err != nil {
		dout.Close()
		chip.Close()
		return nil, fmt.Errorf("failed to request PD_SCK pin %d: %w", pdSckPin, err)
	}
	h.pdSck = pdSck

	logger.Infof("HX711 pins opened: DOUT=%d, PD_SCK=%d on %s", doutPin, pdSckPin, chip)

	// Power cycle the HX711 to ensure it starts in a known state
	pdSck.SetValue(1)
	time.Sleep(100 * time.Microsecond)
	pdSck.SetValue(0)
	time.Sleep(500 * time.Millisecond)

	logger.Infof("HX711 power cycled, gain pulses: %d", h.gain)

	// GPIO self-test
	doutVal, _ := dout.Value()
	logger.Infof("HX711 GPIO self-test: DOUT = %d", doutVal)

	// Do a throwaway read to set the gain
	logger.Info("HX711 doing throwaway read to set gain")
	if _, err := h.readRawBytes(); err != nil {
		return nil, fmt.Errorf("initial read failed: %w", err)
	}
	logger.Info("HX711 initialization complete")

	return h, nil
}

// readBit clocks PD_SCK and reads one bit from DOUT.
// Matches the HX711 reference driver: read DOUT after falling edge.
// Data is stable from 0.1μs after rising edge until the next rising edge.
// Reading after falling edge minimizes PD_SCK high time (~10μs for one ioctl
// vs ~20μs if we read while high), reducing risk of >60μs power-down.
func (h *HX711) readBit() (int, error) {
	h.pdSck.SetValue(1)
	h.pdSck.SetValue(0)
	val, err := h.dout.Value()
	return val, err
}

// readByte reads 8 bits MSB-first and returns a byte value.
func (h *HX711) readByte() (byte, error) {
	var val byte
	for i := 0; i < 8; i++ {
		bit, err := h.readBit()
		if err != nil {
			return 0, err
		}
		val <<= 1
		if bit != 0 {
			val |= 1
		}
	}
	return val, nil
}

// readRawBytes reads 3 bytes (24 bits) from the HX711 plus gain-setting pulses.
func (h *HX711) readRawBytes() ([3]byte, error) {
	// Wait for DOUT to go low (data ready), with timeout
	deadline := time.Now().Add(5 * time.Second)
	waitCount := 0
	for {
		val, err := h.dout.Value()
		if err != nil {
			return [3]byte{}, fmt.Errorf("failed to read DOUT: %w", err)
		}
		if val == 0 {
			break // DOUT low means ready
		}
		waitCount++
		if time.Now().After(deadline) {
			return [3]byte{}, fmt.Errorf("HX711 not ready: DOUT pin never went low after %d checks", waitCount)
		}
		time.Sleep(100 * time.Microsecond)
	}
	h.logger.Debugf("HX711 ready after %d waits", waitCount)

	// Lock OS thread during bit-banging to prevent goroutine preemption.
	// If PD_SCK stays HIGH >60μs (due to context switch), the HX711 resets.
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()

	// Read 3 bytes of data
	var data [3]byte
	var err error
	data[0], err = h.readByte()
	if err != nil {
		return [3]byte{}, fmt.Errorf("read byte 0: %w", err)
	}
	data[1], err = h.readByte()
	if err != nil {
		return [3]byte{}, fmt.Errorf("read byte 1: %w", err)
	}
	data[2], err = h.readByte()
	if err != nil {
		return [3]byte{}, fmt.Errorf("read byte 2: %w", err)
	}

	// Send extra clock pulses to set gain for next read
	for i := 0; i < h.gain; i++ {
		if _, err := h.readBit(); err != nil {
			return [3]byte{}, fmt.Errorf("gain pulse: %w", err)
		}
	}

	h.logger.Debugf("HX711 raw bytes: [0x%02x, 0x%02x, 0x%02x]", data[0], data[1], data[2])
	return data, nil
}

// ReadLong reads a single 24-bit signed value from the HX711.
func (h *HX711) ReadLong() (int32, error) {
	h.mu.Lock()
	defer h.mu.Unlock()

	data, err := h.readRawBytes()
	if err != nil {
		return 0, err
	}

	// Join bytes into 24-bit value
	raw := uint32(data[0])<<16 | uint32(data[1])<<8 | uint32(data[2])

	// Convert from 24-bit two's complement
	signed := int32(raw&0x7fffff) - int32(raw&0x800000)

	h.logger.Debugf("HX711 raw=0x%06x signed=%d", raw, signed)
	return signed, nil
}

// ReadAverage takes n readings, trims 20% outliers, and returns the mean.
func (h *HX711) ReadAverage(n int) (float64, error) {
	if n <= 0 {
		return 0, fmt.Errorf("samples must be >= 1")
	}
	if n == 1 {
		v, err := h.ReadLong()
		return float64(v), err
	}

	values := make([]float64, n)
	for i := 0; i < n; i++ {
		v, err := h.ReadLong()
		if err != nil {
			return 0, err
		}
		values[i] = float64(v)
	}

	h.logger.Debugf("ReadAverage raw: %v", logging.FloatArrayFormat{"", values})

	return average(values), nil
}

// PowerDown puts the HX711 into low-power mode.
func (h *HX711) PowerDown() {
	h.mu.Lock()
	defer h.mu.Unlock()

	h.pdSck.SetValue(0)
	h.pdSck.SetValue(1)
	time.Sleep(100 * time.Microsecond)
}

// Close releases GPIO resources.
func (h *HX711) Close() {
	h.PowerDown()
	h.dout.Close()
	h.pdSck.Close()
}
