package config

import (
	"fmt"
	"time"
)

type Duration time.Duration

func (d *Duration) UnmarshalYAML(unmarshal func(any) error) error {
	var s string
	err := unmarshal(&s)
	if err != nil {
		return err
	}

	duration, err := time.ParseDuration(s)
	if err != nil {
		return fmt.Errorf("couldn't parse duration: %w", err)
	}

	*d = Duration(duration)
	return nil
}

func (d *Duration) Duration() time.Duration {
	return time.Duration(*d)
}
