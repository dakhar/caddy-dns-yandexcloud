// Package yandexcloud implements a Caddy DNS provider (libdns) for Yandex Cloud DNS,
// for use with the ACME DNS-01 challenge.
//
// Портировано из github.com/profcomff/{libdns,caddy-dns}-yandex-cloud (MIT) под
// актуальный libdns v1.x, где Record стал интерфейсом (было — struct). Изменения:
// маппинг записей через libdns.RR{Name,Type,TTL,Data} и чтение входных записей
// через .RR(). Обёртка Caddy и провайдер объединены в один пакет/модуль (вендорится
// в репо, xcaddy собирает из локального пути).
package yandexcloud

import (
	"context"
	"strings"

	"github.com/libdns/libdns"
)

// Provider реализует libdns-интерфейсы для Yandex Cloud DNS и является Caddy-модулем.
type Provider struct {
	// Путь к файлу авторизованного ключа сервисного аккаунта YC (JSON из
	// `yc iam key create`). Должен содержать также dns_zone_id (см. serviceConfig).
	ServiceAccountConfigPath string `json:"service_account_config_path,omitempty"`

	serviceConfigParsed serviceConfig
	authAPIToken        string
}

func (p *Provider) updateAPIToken() error {
	if p.serviceConfigParsed == (serviceConfig{}) {
		if err := parseServiceConfig(p.ServiceAccountConfigPath, &p.serviceConfigParsed); err != nil {
			return err
		}
	}
	token, err := getIAMToken(p.serviceConfigParsed)
	if err != nil {
		return err
	}
	p.authAPIToken = token
	return nil
}

// GetRecords lists all the records in the zone.
func (p *Provider) GetRecords(ctx context.Context, zone string) ([]libdns.Record, error) {
	if err := p.updateAPIToken(); err != nil {
		return nil, err
	}
	return getAllRecords(ctx, p.authAPIToken, p.serviceConfigParsed.DnsZoneId)
}

// AppendRecords adds records to the zone and returns the records that were added.
func (p *Provider) AppendRecords(ctx context.Context, zone string, records []libdns.Record) ([]libdns.Record, error) {
	if err := p.updateAPIToken(); err != nil {
		return nil, err
	}
	return updateRecords(ctx, p.authAPIToken, p.serviceConfigParsed.DnsZoneId, records, "ADD")
}

// DeleteRecords deletes the records from the zone.
func (p *Provider) DeleteRecords(ctx context.Context, zone string, records []libdns.Record) ([]libdns.Record, error) {
	if err := p.updateAPIToken(); err != nil {
		return nil, err
	}
	if _, err := updateRecords(ctx, p.authAPIToken, p.serviceConfigParsed.DnsZoneId, records, "DELETE"); err != nil {
		return nil, err
	}
	return records, nil
}

// SetRecords sets the records in the zone (upsert), returning the input records.
func (p *Provider) SetRecords(ctx context.Context, zone string, records []libdns.Record) ([]libdns.Record, error) {
	if err := p.updateAPIToken(); err != nil {
		return nil, err
	}
	if _, err := upsertRecords(ctx, p.authAPIToken, p.serviceConfigParsed.DnsZoneId, records, "MERGE"); err != nil {
		return nil, err
	}
	return records, nil
}

func unFQDN(fqdn string) string {
	return strings.TrimSuffix(fqdn, ".")
}

// Interface guards
var (
	_ libdns.RecordGetter   = (*Provider)(nil)
	_ libdns.RecordAppender = (*Provider)(nil)
	_ libdns.RecordSetter   = (*Provider)(nil)
	_ libdns.RecordDeleter  = (*Provider)(nil)
)
