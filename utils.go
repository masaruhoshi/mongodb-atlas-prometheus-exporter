package main

import (
	"time"
)

func DiffS(dt string) (time.Duration, error) {
	now := time.Now()

	created, err := time.Parse(time.RFC3339, dt)
	if err != nil {
		return 0, err
	}

	diff := now.Sub(created)
	return diff, nil
}
