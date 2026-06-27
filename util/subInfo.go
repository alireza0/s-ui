package util

import (
	"fmt"

	"github.com/alireza0/s-ui/database/model"
)

func GetHeaders(client *model.Client, updateInterval int) []string {
	var headers []string
	headers = append(headers, fmt.Sprintf("upload=%d; download=%d; total=%d; expire=%d", client.Up, client.Down, client.Volume, client.Expiry))
	headers = append(headers, fmt.Sprintf("%d", updateInterval))
	// Profile-Title: client remark when set, otherwise the client name
	title := client.Remark
	if title == "" {
		title = client.Name
	}
	headers = append(headers, title)
	return headers
}
