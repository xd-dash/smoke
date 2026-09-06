package redisprovider

import "testing"

func TestSubscriptionKinds(t *testing.T) {
	tests := []struct {
		name     string
		channels []string
		patterns []string
		want     subscriptionKind
	}{
		{name: "channels", channels: []string{"events"}, want: subscriptionChannels},
		{name: "patterns", patterns: []string{"worker:*"}, want: subscriptionPatterns},
		{name: "mixed", channels: []string{"events"}, patterns: []string{"worker:*"}, want: subscriptionMixed},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := classifySubscription(tt.channels, tt.patterns); got != tt.want {
				t.Fatalf("classifySubscription() = %v, want %v", got, tt.want)
			}
		})
	}
}
