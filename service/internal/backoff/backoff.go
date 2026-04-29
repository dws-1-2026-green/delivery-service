package backoff

import "time"

type Config struct {
	BaseDelay   time.Duration
	MaxDelay    time.Duration
	MaxAttempts int
}

// Delay returns the wait duration before the next attempt.
// attempts is the number of attempts already made.
// Formula: min(BaseDelay * 2^attempts, MaxDelay)
func (c Config) Delay(attempts int) time.Duration {
	delay := c.BaseDelay
	for i := 0; i < attempts; i++ {
		delay *= 2
		if delay <= 0 || delay > c.MaxDelay {
			return c.MaxDelay
		}
	}
	return delay
}
