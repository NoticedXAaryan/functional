package cases

import (
	"fmt"
	"time"
	_ "time/tzdata"
)

type ContactPreference struct {
	PreferredChannel string `json:"preferred_channel"`
	SafeHoursStart   string `json:"safe_hours_start"`
	SafeHoursEnd     string `json:"safe_hours_end"`
	TimeZone         string `json:"time_zone"`
}

func (c *ContactPreference) Validate() error {
	if c.PreferredChannel == "" {
		c.PreferredChannel = "message_in_app"
	}
	if c.PreferredChannel != "message_in_app" && c.PreferredChannel != "no_contact" {
		return fmt.Errorf("Choose in-app messages or no contact")
	}
	if c.SafeHoursStart == "" {
		c.SafeHoursStart = "00:00"
	}
	if c.SafeHoursEnd == "" {
		c.SafeHoursEnd = "00:00"
	}
	if _, err := time.Parse("15:04", c.SafeHoursStart); err != nil {
		return fmt.Errorf("Use HH:MM for the start time")
	}
	if _, err := time.Parse("15:04", c.SafeHoursEnd); err != nil {
		return fmt.Errorf("Use HH:MM for the end time")
	}
	if c.TimeZone == "" {
		c.TimeZone = "Asia/Kolkata"
	}
	if c.TimeZone != "Asia/Kolkata" && c.TimeZone != "UTC" {
		return fmt.Errorf("Use Asia/Kolkata or UTC for this beta")
	}
	if _, err := time.LoadLocation(c.TimeZone); err != nil {
		return fmt.Errorf("Choose a valid time zone")
	}
	return nil
}
