package callback

import (
	"fmt"
	"net/url"
	"strings"
)

// Parse converts repeated callback= query values into callbacks. Supported
// values are "stdout" and absolute http(s) webhook URLs.
func Parse(values []string) (*Dispatcher, error) {
	callbacks := make([]Callback, 0, len(values))

	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}
		if value == "stdout" {
			callbacks = append(callbacks, Stdout{})
			continue
		}

		u, err := url.Parse(value)
		if err != nil || u.Host == "" || (u.Scheme != "http" && u.Scheme != "https") {
			return nil, fmt.Errorf("unsupported callback %q", value)
		}
		callbacks = append(callbacks, Webhook{URL: u})
	}

	return New(callbacks...), nil
}
