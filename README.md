# caddy-dns-yandexcloud

Caddy DNS provider (libdns) для **Yandex Cloud DNS** — для ACME DNS-01 challenge.

Порт [`profcomff/libdns-yandex-cloud`](https://github.com/profcomff/libdns-yandex-cloud)
+ [`profcomff/caddy-dns-yandex-cloud`](https://github.com/profcomff/caddy-dns-yandex-cloud)
(MIT) под актуальный **libdns v1.x** (Caddy 2.11+), где `libdns.Record` стал интерфейсом.
Провайдер и Caddy-обёртка объединены в один модуль.

## Сборка

```bash
xcaddy build --with github.com/dakhar/caddy-dns-yandexcloud
```

## Caddyfile

```caddyfile
example.com {
    tls {
        dns yandex_cloud {env.YCLOUD_KEYS_FILE}   # путь к JSON-ключу SA
    }
}
```
или глобально:
```caddyfile
{
    acme_dns yandex_cloud /etc/caddy/yc-sa-key.json
}
```

## Учётные данные

Файл авторизованного ключа сервисного аккаунта YC (JSON из `yc iam key create`),
**плюс поле `dns_zone_id`** зоны:

```json
{
  "id": "<key-id>",
  "service_account_id": "<sa-id>",
  "private_key": "-----BEGIN PRIVATE KEY-----\n...\n",
  "dns_zone_id": "<zone-id>"
}
```

SA нужна роль `dns.editor` на folder/зону.

## License

MIT — см. [LICENSE](LICENSE) (© Профком студентов физфака МГУ, оригинал; порт под libdns v1.x).
