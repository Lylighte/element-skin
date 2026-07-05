package homepage

import (
	"net/http"
	"strconv"
	"strings"

	"element-skin/backend/internal/util"
)

type UploadInput struct {
	Filename string
	Data     []byte
	Fields   map[string]string
}

func ParseMediaValues(fields map[string]string, typ string) (MediaValues, error) {
	values := MediaValues{
		StartYaw:      0,
		StartPitch:    0,
		YawSpeedDPS:   4,
		PitchSpeedDPS: 0,
		DurationMS:    intField(fields, "duration_ms", 0),
	}
	var err error
	values.OverlayOpacityLight, err = floatField(fields, "overlay_opacity_light", 0.45)
	if err != nil {
		return MediaValues{}, util.HTTPError{Status: http.StatusBadRequest, Detail: "overlay_opacity_light must be a number"}
	}
	values.OverlayOpacityDark, err = floatField(fields, "overlay_opacity_dark", 0.45)
	if err != nil {
		return MediaValues{}, util.HTTPError{Status: http.StatusBadRequest, Detail: "overlay_opacity_dark must be a number"}
	}
	if err := validateOpacityValue("overlay_opacity_light", values.OverlayOpacityLight); err != nil {
		return MediaValues{}, err
	}
	if err := validateOpacityValue("overlay_opacity_dark", values.OverlayOpacityDark); err != nil {
		return MediaValues{}, err
	}
	if typ != "panorama" {
		return values, nil
	}
	if values.StartYaw, err = floatField(fields, "start_yaw", 0); err != nil {
		return MediaValues{}, util.HTTPError{Status: http.StatusBadRequest, Detail: "start_yaw must be a number"}
	}
	if values.StartPitch, err = floatField(fields, "start_pitch", 0); err != nil {
		return MediaValues{}, util.HTTPError{Status: http.StatusBadRequest, Detail: "start_pitch must be a number"}
	}
	if values.YawSpeedDPS, err = floatField(fields, "yaw_speed_dps", 4); err != nil {
		return MediaValues{}, util.HTTPError{Status: http.StatusBadRequest, Detail: "yaw_speed_dps must be a number"}
	}
	if values.PitchSpeedDPS, err = floatField(fields, "pitch_speed_dps", 0); err != nil {
		return MediaValues{}, util.HTTPError{Status: http.StatusBadRequest, Detail: "pitch_speed_dps must be a number"}
	}
	if err := ValidatePanoramaValues(&values.StartYaw, &values.StartPitch, &values.YawSpeedDPS, &values.PitchSpeedDPS); err != nil {
		return MediaValues{}, err
	}
	return values, nil
}

func ValidateOpacity(name string, v *float64) error {
	if v == nil {
		return nil
	}
	return validateOpacityValue(name, *v)
}

func validateOpacityValue(name string, v float64) error {
	if v < 0 || v > 0.9 {
		return util.HTTPError{Status: http.StatusBadRequest, Detail: name + " out of range"}
	}
	return nil
}

func ValidatePanoramaValues(startYaw, startPitch, yawSpeedDPS, pitchSpeedDPS *float64) error {
	if startYaw != nil && (*startYaw < -360 || *startYaw > 360) {
		return util.HTTPError{Status: http.StatusBadRequest, Detail: "start_yaw out of range"}
	}
	if startPitch != nil && (*startPitch < -89 || *startPitch > 89) {
		return util.HTTPError{Status: http.StatusBadRequest, Detail: "start_pitch out of range"}
	}
	if yawSpeedDPS != nil && (*yawSpeedDPS < -90 || *yawSpeedDPS > 90) {
		return util.HTTPError{Status: http.StatusBadRequest, Detail: "yaw_speed_dps out of range"}
	}
	if pitchSpeedDPS != nil && (*pitchSpeedDPS < -90 || *pitchSpeedDPS > 90) {
		return util.HTTPError{Status: http.StatusBadRequest, Detail: "pitch_speed_dps out of range"}
	}
	return nil
}

func floatField(fields map[string]string, key string, fallback float64) (float64, error) {
	raw := strings.TrimSpace(fields[key])
	if raw == "" {
		return fallback, nil
	}
	return strconv.ParseFloat(raw, 64)
}

func intField(fields map[string]string, key string, fallback int) int {
	raw := strings.TrimSpace(fields[key])
	if raw == "" {
		return fallback
	}
	n, err := strconv.Atoi(raw)
	if err != nil {
		return fallback
	}
	return n
}
