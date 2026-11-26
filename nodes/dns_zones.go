package nodes

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/alireza0/s-ui/database"
	"github.com/alireza0/s-ui/database/model"
	"net/mail"
	"strings"
)

func DiscoverDNSZones(ctx context.Context, data json.RawMessage) ([]DNSZone, error) {
	var r struct {
		Id       uint   `json:"id"`
		AuthType string `json:"auth_type"`
		Email    string `json:"email"`
		Token    string `json:"token"`
		Key      string `json:"api_key"`
	}
	if err := decodeJSON(data, &r, true); err != nil {
		return nil, err
	}
	p := model.NodeDNSProfile{Id: r.Id, AuthType: r.AuthType, Email: r.Email}
	secret := r.Token
	if r.AuthType == "global_key" {
		secret = r.Key
	}
	if secret == "" && r.Id > 0 {
		var stored model.NodeDNSProfile
		if err := database.GetDB().First(&stored, r.Id).Error; err != nil {
			return nil, err
		}
		if stored.AuthType != r.AuthType || stored.Email != r.Email {
			return nil, errors.New("dns_credentials_required")
		}
		var err error
		secret, err = profileSecret(stored)
		if err != nil {
			return nil, err
		}
	}
	if secret == "" || len(secret) > 4096 || strings.ContainsAny(secret, "\r\n\x00") {
		return nil, errors.New("invalid_dns_credentials")
	}
	if r.AuthType != "token" && r.AuthType != "global_key" {
		return nil, errors.New("invalid_dns_auth_type")
	}
	if r.AuthType == "global_key" {
		if _, err := mail.ParseAddress(r.Email); err != nil {
			return nil, errors.New("dns_email_required")
		}
	}
	result := []DNSZone{}
	for page := 1; page <= 8; page++ {
		var zones []DNSZone
		if err := cfResourceAuth(ctx, p, secret, "GET", fmt.Sprintf("/zones?per_page=50&page=%d", page), nil, &zones); err != nil {
			return nil, err
		}
		result = append(result, zones...)
		if len(zones) < 50 {
			return result, nil
		}
	}
	return nil, errors.New("dns_zone_inventory_too_large")
}
