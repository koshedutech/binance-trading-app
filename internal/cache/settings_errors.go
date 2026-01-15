package cache

import "errors"

var (
	// ErrCacheUnavailable is returned when Redis is not healthy
	ErrCacheUnavailable = errors.New("cache unavailable - Redis is not healthy")

	// ErrSettingNotFound is returned when a setting doesn't exist in cache or DB
	ErrSettingNotFound = errors.New("setting not found")
)
