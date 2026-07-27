package artifacts

import "fmt"

type Platform struct {
	OS           string
	Architecture string
}

func (p Platform) Key() string {
	return p.OS + "-" + p.Architecture
}

func Select(available map[string]string, platform Platform) (string, error) {
	if artifact := available[platform.Key()]; artifact != "" {
		return artifact, nil
	}
	return "", fmt.Errorf("no artifact supports %s/%s", platform.OS, platform.Architecture)
}
