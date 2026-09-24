package helpers

import (
	"testing"
	"testing/synctest"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGetTimestampFromDeploymentID(t *testing.T) {
	tests := []struct {
		name         string
		deploymentID string
		wantErr      bool
	}{
		{
			name:         "empty_string",
			deploymentID: "",
			wantErr:      true,
		},
		{
			name:         "too_short",
			deploymentID: "123",
			wantErr:      true,
		},
		{
			name:         "contains_invalid_char_exclamation",
			deploymentID: "01ARZ3NDEKTSV4RRFFQ69G5FA!",
			wantErr:      true,
		},
		{
			name:         "contains_invalid_char_letter_o",
			deploymentID: "01ARZ3NDEKTSV4RRFFQ69G5FAO",
			wantErr:      true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := GetTimestampFromDeploymentID(tt.deploymentID)

			if tt.wantErr {
				assert.Error(t, err)
				assert.True(t, result.IsZero())
			} else {
				assert.NoError(t, err)
				assert.False(t, result.IsZero())
			}
		})
	}
}

func TestGetTimestampFromDeploymentID_ValidULID(t *testing.T) {
	result, err := GetTimestampFromDeploymentID("01ARZ3NDEKTSV4RRFFQ69G5FAV")
	require.NoError(t, err)
	want := time.Date(2016, time.July, 30, 23, 54, 10, 259_000_000, time.UTC)
	if !result.Equal(want) {
		t.Fatalf("GetTimestampFromDeploymentID() = %v, want %v", result, want)
	}
}

func TestFormatTime(t *testing.T) {
	const day = 24 * time.Hour
	tests := []struct {
		elapsed time.Duration
		want    string
	}{
		{0, "1 second ago"},
		{time.Second, "1 second ago"},
		{2 * time.Second, "2 seconds ago"},
		{time.Minute - time.Nanosecond, "59 seconds ago"},
		{time.Minute, "1 minute ago"},
		{2 * time.Minute, "2 minutes ago"},
		{time.Hour - time.Nanosecond, "59 minutes ago"},
		{time.Hour, "1 hour ago"},
		{2 * time.Hour, "2 hours ago"},
		{day - time.Nanosecond, "23 hours ago"},
		{day, "1 day ago"},
		{2 * day, "2 days ago"},
		{30*day - time.Second, "29 days ago"},
		{30 * day, "1 month ago"},
		{60 * day, "2 months ago"},
		{365*day - time.Second, "12 months ago"},
		{365 * day, "1 year ago"},
		{2 * 365 * day, "2 years ago"},
		{-time.Second, "1 second from now"},
		{-2 * time.Hour, "2 hours from now"},
	}

	for _, tt := range tests {
		t.Run(tt.elapsed.String(), func(t *testing.T) {
			synctest.Test(t, func(t *testing.T) {
				got := FormatTime(time.Now().Add(-tt.elapsed))
				if got != tt.want {
					t.Fatalf("FormatTime() = %q, want %q", got, tt.want)
				}
			})
		})
	}
}
