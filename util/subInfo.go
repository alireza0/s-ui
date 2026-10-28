package util

import (
	"fmt"
	"strings"
	"unicode"

	"github.com/alireza0/s-ui/database/model"
)

func GetHeaders(client *model.Client, updateInterval int) []string {
	var headers []string
	headers = append(headers, fmt.Sprintf("upload=%d; download=%d; total=%d; expire=%d", client.Up, client.Down, client.Volume, client.Expiry))
	headers = append(headers, fmt.Sprintf("%d", updateInterval))
	// Profile-Title: client remark when set, otherwise the client name.
	// Neither has any character restriction on the way in, but the title
	// becomes an HTTP header value, and Go's net/http refuses to write a
	// response whose header values contain control bytes -- one remark
	// with a CR/LF in it silently breaks every format of that client's
	// subscription. Strip control characters here, at the source; the
	// rest of the subscription output is sanitized the same way
	// (Content-Disposition encoding, custom-header validation).
	title := client.Remark
	if title == "" {
		title = client.Name
	}
	title = strings.Map(func(r rune) rune {
		if unicode.IsControl(r) {
			return -1
		}
		return r
	}, title)
	headers = append(headers, title)
	return headers
}
