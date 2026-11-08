package middleware

import (
	"net"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
)

func DomainValidator(domain string) gin.HandlerFunc {
	return func(c *gin.Context) {
		host := c.Request.Host
		if h, _, err := net.SplitHostPort(host); err == nil {
			host = h
		}
		// An IPv6 host on the default port has no port and keeps its brackets.
		host = strings.TrimSuffix(strings.TrimPrefix(host, "["), "]")

		// Host names are case-insensitive, as in the TLS server name check.
		if !strings.EqualFold(host, domain) {
			c.AbortWithStatus(http.StatusForbidden)
			return
		}

		c.Next()
	}
}
