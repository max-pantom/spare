package isolation

import (
	"os"
	"strings"
)

func cleanEnvironment() []string {
	allowed := map[string]bool{
		"COMSPEC": true, "LANG": true, "PATH": true, "PATHEXT": true,
		"SSL_CERT_DIR": true, "SSL_CERT_FILE": true, "SYSTEMROOT": true,
		"TEMP": true, "TMP": true, "TMPDIR": true, "TZ": true, "WINDIR": true,
	}
	result := []string{}
	for _, entry := range os.Environ() {
		name, _, _ := strings.Cut(entry, "=")
		upper := strings.ToUpper(name)
		if allowed[upper] || strings.HasPrefix(upper, "LC_") {
			result = append(result, entry)
		}
	}
	return result
}
