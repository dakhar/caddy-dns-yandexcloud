package yandexcloud

import (
	"bytes"
	"context"
	"crypto/rsa"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httputil"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/libdns/libdns"
)

type serviceConfig struct {
	ID               string `json:"id"`
	PrivateKey       string `json:"private_key"`
	ServiceAccountID string `json:"service_account_id"`
	DnsZoneId        string `json:"dns_zone_id"`
}

type getAllRecordsResponse struct {
	Records []record `json:"recordSets"`
}

type createRecordResponse struct {
	ID       string            `json:"id,omitempty"`
	Response upsertRecordsBody `json:"response"`
	IsDone   bool              `json:"done"`
}

type updateRecordResponse struct {
	ID       string            `json:"id,omitempty"`
	Response updateRecordsBody `json:"response"`
	IsDone   bool              `json:"done"`
}

type upsertRecordsBody struct {
	Deletions    []record `json:"deletions"`
	Replacements []record `json:"replacements"`
	Merges       []record `json:"merges"`
}

type updateRecordsBody struct {
	Deletions []record `json:"deletions"`
	Additions []record `json:"additions"`
}

type record struct {
	ID   string   `json:"id,omitempty"`
	Type string   `json:"type"`
	Name string   `json:"name"`
	Data []string `json:"data"`
	TTL  string   `json:"ttl"`
}

type zone struct {
	ID   string `json:"id,omitempty"`
	Zone string `json:"zone,omitempty"`
	Name string `json:"name"`
}

func doRequest(token string, request *http.Request) ([]byte, error) {
	request.Header.Add("Authorization", fmt.Sprintf("Bearer %s", token))

	client := &http.Client{}
	reqDump, _ := httputil.DumpRequestOut(request, true)

	response, err := client.Do(request)
	if err != nil {
		return nil, err
	}
	defer response.Body.Close()

	if response.StatusCode < 200 || response.StatusCode >= 300 {
		resDump, _ := httputil.DumpResponse(response, true)
		return nil, fmt.Errorf("%s\n%s", string(resDump), string(reqDump))
	}

	data, err := io.ReadAll(response.Body)
	if err != nil {
		return nil, err
	}
	return data, nil
}

func getZoneName(ctx context.Context, token string, zoneID string) (string, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", fmt.Sprintf("https://dns.api.cloud.yandex.net/dns/v1/zones/%s", zoneID), nil)
	if err != nil {
		return "", err
	}
	data, err := doRequest(token, req)
	if err != nil {
		return "", err
	}
	result := zone{}
	if err := json.Unmarshal(data, &result); err != nil {
		return "", err
	}
	return unFQDN(result.Zone), nil
}

func getAllRecords(ctx context.Context, token string, zoneID string) ([]libdns.Record, error) {
	url := fmt.Sprintf("https://dns.api.cloud.yandex.net/dns/v1/zones/%s:listRecordSets", zoneID)
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, err
	}
	data, err := doRequest(token, req)
	if err != nil {
		return nil, err
	}
	result := getAllRecordsResponse{}
	if err := json.Unmarshal(data, &result); err != nil {
		return nil, err
	}

	records := []libdns.Record{}
	for _, r := range result.Records {
		intTtl, err := strconv.Atoi(r.TTL)
		if err != nil {
			return nil, err
		}
		// В recordSet может быть несколько значений — возвращаем по записи на каждое.
		for _, val := range r.Data {
			records = append(records, libdns.RR{
				Name: r.Name,
				Type: r.Type,
				TTL:  time.Duration(intTtl) * time.Second,
				Data: val,
			})
		}
	}
	return records, nil
}

func upsertRecords(ctx context.Context, token string, zoneID string, rs []libdns.Record, method string) ([]libdns.Record, error) {
	reqData := upsertRecordsBody{
		Replacements: []record{},
		Deletions:    []record{},
		Merges:       []record{},
	}
	zoneName, err := getZoneName(ctx, token, zoneID)
	if err != nil {
		return nil, err
	}
	for _, rec := range rs {
		rr := rec.RR()
		recordData := record{
			Type: rr.Type,
			Name: normalizeRecordName(rr.Name, zoneName),
			Data: []string{rr.Data},
			TTL:  fmt.Sprint(rr.TTL.Seconds()),
		}
		switch method {
		case "MERGE":
			reqData.Merges = append(reqData.Merges, recordData)
		case "REPLACE":
			reqData.Replacements = append(reqData.Replacements, recordData)
		case "DELETE":
			reqData.Deletions = append(reqData.Deletions, recordData)
		}
	}
	reqBuffer, err := json.Marshal(reqData)
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequestWithContext(ctx, "POST", fmt.Sprintf("https://dns.api.cloud.yandex.net/dns/v1/zones/%s:upsertRecordSets", zoneID), bytes.NewBuffer(reqBuffer))
	if err != nil {
		return nil, err
	}
	data, err := doRequest(token, req)
	if err != nil {
		return nil, err
	}
	result := createRecordResponse{}
	if err := json.Unmarshal(data, &result); err != nil {
		return nil, err
	}
	// YC upsertRecordSets не возвращает записи в ответе — отдаём вход.
	return rs, nil
}

func updateRecords(ctx context.Context, token string, zoneID string, rs []libdns.Record, method string) ([]libdns.Record, error) {
	zoneName, err := getZoneName(ctx, token, zoneID)
	if err != nil {
		return nil, err
	}
	reqData := updateRecordsBody{
		Additions: []record{},
		Deletions: []record{},
	}
	for _, rec := range rs {
		rr := rec.RR()
		recordData := record{
			Type: rr.Type,
			Name: normalizeRecordName(rr.Name, zoneName),
			Data: []string{rr.Data},
			TTL:  fmt.Sprint(rr.TTL.Seconds()),
		}
		switch method {
		case "ADD":
			reqData.Additions = append(reqData.Additions, recordData)
		case "DELETE":
			reqData.Deletions = append(reqData.Deletions, recordData)
		}
	}
	reqBuffer, err := json.Marshal(reqData)
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequestWithContext(ctx, "POST", fmt.Sprintf("https://dns.api.cloud.yandex.net/dns/v1/zones/%s:updateRecordSets", zoneID), bytes.NewBuffer(reqBuffer))
	if err != nil {
		return nil, err
	}
	data, err := doRequest(token, req)
	if err != nil {
		return nil, err
	}
	result := updateRecordResponse{}
	if err := json.Unmarshal(data, &result); err != nil {
		return nil, err
	}
	resultList := []record{}
	switch method {
	case "DELETE":
		resultList = result.Response.Deletions
	case "ADD":
		resultList = result.Response.Additions
	}
	res := make([]libdns.Record, 0)
	for _, r := range resultList {
		intTtl, _ := strconv.Atoi(r.TTL)
		for _, val := range r.Data {
			res = append(res, libdns.RR{
				Name: normalizeRecordName(r.Name, zoneName),
				Type: r.Type,
				TTL:  time.Duration(intTtl) * time.Second,
				Data: val,
			})
		}
	}
	return res, nil
}

func normalizeRecordName(recordName string, zone string) string {
	normalized := unFQDN(recordName)
	normalized = strings.TrimSuffix(normalized, unFQDN(zone))
	return unFQDN(normalized)
}

func loadPrivateKey(cfg serviceConfig) (*rsa.PrivateKey, error) {
	return jwt.ParseRSAPrivateKeyFromPEM([]byte(cfg.PrivateKey))
}

func signedToken(cfg serviceConfig) (string, error) {
	claims := jwt.RegisteredClaims{
		Issuer:    cfg.ServiceAccountID,
		ExpiresAt: jwt.NewNumericDate(time.Now().UTC().Add(1 * time.Hour)),
		IssuedAt:  jwt.NewNumericDate(time.Now().UTC()),
		NotBefore: jwt.NewNumericDate(time.Now().UTC()),
		Audience:  []string{"https://iam.api.cloud.yandex.net/iam/v1/tokens"},
	}
	token := jwt.NewWithClaims(jwt.SigningMethodPS256, claims)
	token.Header["kid"] = cfg.ID

	privateKey, err := loadPrivateKey(cfg)
	if err != nil {
		return "", err
	}
	return token.SignedString(privateKey)
}

func parseServiceConfig(keyFile string, sConf *serviceConfig) error {
	data, err := os.ReadFile(keyFile)
	if err != nil {
		return err
	}
	return json.Unmarshal(data, sConf)
}

func getIAMToken(cfg serviceConfig) (string, error) {
	jot, err := signedToken(cfg)
	if err != nil {
		return "", err
	}
	resp, err := http.Post(
		"https://iam.api.cloud.yandex.net/iam/v1/tokens",
		"application/json",
		strings.NewReader(fmt.Sprintf(`{"jwt":"%s"}`, jot)),
	)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("iam token request: %s: %s", resp.Status, body)
	}
	var out struct {
		IAMToken string `json:"iamToken"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return "", err
	}
	return out.IAMToken, nil
}
