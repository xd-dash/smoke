package callback

import (
	"fmt"
	"net/url"
	"strings"
)

// Parse converts repeated callback values into callbacks. Supported values are
// "stdout", absolute http(s) webhook URLs, and axiom:// dataset callbacks.
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
		if err != nil {
			return nil, fmt.Errorf("unsupported callback %q", value)
		}
		switch u.Scheme {
		case "http", "https":
			if u.Host == "" {
				return nil, fmt.Errorf("unsupported callback %q", value)
			}
			callbacks = append(callbacks, Webhook{URL: u})
		case "axiom":
			cb, err := parseAxiomURL(u)
			if err != nil {
				return nil, fmt.Errorf("callback %q: %w", value, err)
			}
			callbacks = append(callbacks, cb)
		default:
			return nil, fmt.Errorf("unsupported callback %q", value)
		}
	}

	return New(callbacks...), nil
}
