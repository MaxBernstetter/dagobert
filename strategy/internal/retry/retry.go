package retry

import "time"

const attempts = 3

// Do runs fn up to three times with a short pause between failures.
func Do(fn func() error) error {
	var last error
	for i := 0; i < attempts; i++ {
		last = fn()
		if last == nil {
			return nil
		}
		if i < attempts-1 {
			time.Sleep(50 * time.Millisecond)
		}
	}
	return last
}
