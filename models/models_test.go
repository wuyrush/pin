package models

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestModels_PinVisibleTo(t *testing.T) {
	tcs := []struct {
		name     string
		pin      Pin
		user     *User
		expected bool
	}{
		{
			name:     "PinPublicAccessible",
			pin:      Pin{AccessMode: AccessModePublic},
			expected: true,
		},
		{
			name:     "PinPrivateWithNonOwnerAccess",
			pin:      Pin{AccessMode: AccessModePrivate, OwnerID: "bar"},
			user:     &User{ID: "foo"},
			expected: false,
		},
		{
			name:     "PinPrivateWithOwnerAccess",
			pin:      Pin{AccessMode: AccessModePrivate, OwnerID: "bar"},
			user:     &User{ID: "bar"},
			expected: true,
		},
	}
	for _, c := range tcs {
		t.Run(c.name, func(t *testing.T) {
			actual := c.pin.VisibleTo(c.user)
			assert.Equal(t, c.expected, actual, "unexpected pin visibility behavior")
		})
	}
}

func TestModels_PinStale(t *testing.T) {
	tcs := []struct {
		name   string
		pin    Pin
		staled bool
	}{
		{
			name:   "staled",
			pin:    Pin{CreationTime: time.Unix(0, 0)},
			staled: true,
		},
		{
			name:   "fresh",
			pin:    Pin{CreationTime: time.Now(), GoodFor: 300 * time.Minute},
			staled: false,
		},
	}
	for _, c := range tcs {
		t.Run(c.name, func(t *testing.T) {
			assert.Equal(t, c.staled, c.pin.Stale(), "unexpected pin staleness")
		})
	}
}

func TestModels_UserAnonymous(t *testing.T) {
	tcs := []struct {
		user      *User
		anonymous bool
	}{
		{
			anonymous: true,
		},
		{
			user:      &User{ID: "johndoe"},
			anonymous: false,
		},
	}
	for _, c := range tcs {
		assert.Equal(t, c.anonymous, c.user.Anonymous(), "unexpected user anonymonity")
	}
}
