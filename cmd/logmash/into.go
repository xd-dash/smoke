package logmash

import (
	"fmt"
	"net/url"
	"strings"
)

type intoSpec struct {
	Provider string
	Profile  string
	Target   string
}

func (s intoSpec) callbackURL() (string, error) {
	switch strings.ToLower(strings.TrimSpace(s.Provider)) {
	case "axiom":
		profile, err := axiomProfile(s.Profile)
		if err != nil {
			return "", err
		}
		dataset := strings.TrimSpace(s.Target)
		if dataset == "" {
			return "", fmt.Errorf("axiom dataset is required")
		}
		u := &url.URL{Scheme: "axiom", Host: dataset}
		q := u.Query()
		q.Set("profile", profile)
		u.RawQuery = q.Encode()
		return u.String(), nil
	default:
		return "", fmt.Errorf("unsupported --into provider %q", s.Provider)
	}
}

func axiomProfile(profile string) (string, error) {
	profile = strings.TrimSpace(profile)
	switch strings.ToLower(profile) {
	case "default", "axiom":
		return "axiom.logma.sh", nil
	case "east", "us-east", "us-east-1":
		return "axiom-us-east-1.logma.sh", nil
	case "eu", "eu-central", "eu-central-1":
		return "axiom-eu-central-1.logma.sh", nil
	case "":
		return "", fmt.Errorf("axiom profile is required")
	default:
		profile = strings.TrimSuffix(profile, ".")
		if strings.Contains(profile, ".") {
			return profile, nil
		}
		return profile + ".logma.sh", nil
	}
}

func resolveInto(specs []intoSpec) ([]string, error) {
	values := make([]string, 0, len(specs))
	for _, spec := range specs {
		value, err := spec.callbackURL()
		if err != nil {
			return nil, err
		}
		values = append(values, value)
	}
	return values, nil
}
