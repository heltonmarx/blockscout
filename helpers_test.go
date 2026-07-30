package blockscout

import (
	"time"

	"github.com/avast/retry-go/v5"
)

// retryTestOpts returns retry options suitable for unit tests: fast delays, no jitter.
func retryTestOpts() []retry.Option {
	return []retry.Option{
		retry.Attempts(3),
		retry.Delay(10 * time.Millisecond),
		retry.MaxDelay(50 * time.Millisecond),
		retry.DelayType(retry.FixedDelay),
		retry.RetryIf(isRetryable),
	}
}
