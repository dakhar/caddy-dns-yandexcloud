package yandexcloud

import (
	"github.com/caddyserver/caddy/v2"
	"github.com/caddyserver/caddy/v2/caddyconfig/caddyfile"
)

func init() {
	caddy.RegisterModule(Provider{})
}

// CaddyModule returns the Caddy module information.
func (Provider) CaddyModule() caddy.ModuleInfo {
	return caddy.ModuleInfo{
		ID:  "dns.providers.yandex_cloud",
		New: func() caddy.Module { return &Provider{} },
	}
}

// Provision resolves Caddy placeholders (напр. {env.YCLOUD_KEYS_FILE}) в пути к ключу.
func (p *Provider) Provision(ctx caddy.Context) error {
	repl := caddy.NewReplacer()
	p.ServiceAccountConfigPath = repl.ReplaceAll(p.ServiceAccountConfigPath, "")
	return nil
}

// UnmarshalCaddyfile sets up the provider from Caddyfile tokens. Синтаксис:
//
//	yandex_cloud [<service_account_config_path>] {
//	    service_account_config_path <path>
//	}
func (p *Provider) UnmarshalCaddyfile(d *caddyfile.Dispenser) error {
	for d.Next() {
		if d.NextArg() {
			p.ServiceAccountConfigPath = d.Val()
		}
		if d.NextArg() {
			return d.ArgErr()
		}
		for nesting := d.Nesting(); d.NextBlock(nesting); {
			switch d.Val() {
			case "service_account_config_path":
				if !d.NextArg() {
					return d.ArgErr()
				}
				p.ServiceAccountConfigPath = d.Val()
				if d.NextArg() {
					return d.ArgErr()
				}
			default:
				return d.Errf("unrecognized subdirective '%s'", d.Val())
			}
		}
	}
	if p.ServiceAccountConfigPath == "" {
		return d.Err("missing service account config path")
	}
	return nil
}

// Interface guards
var (
	_ caddyfile.Unmarshaler = (*Provider)(nil)
	_ caddy.Provisioner     = (*Provider)(nil)
)
