package config

import (
	"slices"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

func TestOAuthAdminGroupConfig(t *testing.T) {
	const raw = "server:\n  oauth:\n    oidc:\n      client_id: hister\n      client_secret: secret\n      auth_url: https://provider.example/auth\n      admin_groups: [hister-admins, operators]\n      groups_claim: roles\n"
	for _, environment := range []bool{false, true} {
		t.Run(map[bool]string{false: "YAML", true: "environment"}[environment], func(t *testing.T) {
			wantGroups, wantClaim := []string{"hister-admins", "operators"}, "roles"
			if environment {
				t.Setenv("HISTER__SERVER__OAUTH__OIDC__ADMIN_GROUPS", "admins,staff")
				t.Setenv("HISTER__SERVER__OAUTH__OIDC__GROUPS_CLAIM", "app.roles")
				wantGroups, wantClaim = []string{"admins", "staff"}, "app.roles"
			}
			cfg, err := parseConfig([]byte(raw))
			if err != nil {
				t.Fatal(err)
			}
			if err := cfg.validateOAuth(); err != nil {
				t.Fatal(err)
			}
			data, err := yaml.Marshal(cfg.Server.OAuth["oidc"])
			if err != nil {
				t.Fatal(err)
			}
			var saved struct {
				AdminGroups []string `yaml:"admin_groups"`
				GroupsClaim string   `yaml:"groups_claim"`
			}
			if err := yaml.Unmarshal(data, &saved); err != nil {
				t.Fatal(err)
			}
			if !slices.Equal(saved.AdminGroups, wantGroups) || saved.GroupsClaim != wantClaim {
				t.Fatalf("saved groups = %v, claim = %q; want %v, %q", saved.AdminGroups, saved.GroupsClaim, wantGroups, wantClaim)
			}
		})
	}
	if len(CreateDefaultConfig().Server.OAuth) != 0 {
		t.Fatal("default config enables an OAuth provider")
	}
}

func TestValidateOAuthAdminGroups(t *testing.T) {
	for _, tc := range []struct {
		settings string
		wantErr  string
	}{
		{},
		{settings: "admin_groups: []"},
		{settings: "admin_groups: [hister-admins, 'Hister Admins']"},
		{settings: "admin_groups: [hister-admins]\ngroups_claim: roles"},
		{settings: "groups_claim: https://example.com/roles"},
		{settings: "admin_groups: ['']", wantErr: "admin_groups"},
		{settings: "admin_groups: ['   ']", wantErr: "admin_groups"},
		{settings: `admin_groups: ["admins\n"]`, wantErr: "admin_groups"},
		{settings: `admin_groups: ["admins\u0000"]`, wantErr: "admin_groups"},
		{settings: "groups_claim: ' roles'", wantErr: "groups_claim"},
		{settings: "groups_claim: 'roles name'", wantErr: "groups_claim"},
		{settings: `groups_claim: "roles\u0000"`, wantErr: "groups_claim"},
	} {
		for _, provider := range []string{"oidc", "github", "google"} {
			t.Run(provider+"/"+tc.settings, func(t *testing.T) {
				entry := &OAuthEntry{ClientID: "hister", ClientSecret: "secret", AuthURL: "https://provider.example/auth"}
				if err := yaml.Unmarshal([]byte(tc.settings), entry); err != nil {
					t.Fatal(err)
				}
				cfg := CreateDefaultConfig()
				cfg.Server.OAuth = map[string]*OAuthEntry{provider: entry}
				wantErr := tc.wantErr
				if provider != "oidc" && tc.settings != "" && tc.settings != "admin_groups: []" {
					wantErr = "only supported for oidc"
				}
				err := cfg.validateOAuth()
				if wantErr == "" {
					if err != nil {
						t.Fatal(err)
					}
				} else if err == nil || !strings.Contains(err.Error(), wantErr) {
					t.Fatalf("validation error = %v, want %q", err, wantErr)
				}
			})
		}
	}
}

func TestOAuthPKCEConfig(t *testing.T) {
	const raw = "server:\n  oauth:\n    oidc:\n      client_id: hister\n      client_secret: secret\n      auth_url: https://provider.example/auth\n"
	for _, setting := range []string{"", "      disable_pkce: false\n", "      disable_pkce: true\n"} {
		cfg, err := parseConfig([]byte(raw + setting))
		if err != nil {
			t.Fatal(err)
		}
		wantDisabled := setting == "      disable_pkce: true\n"
		if cfg.Server.OAuth["oidc"].DisablePKCE != wantDisabled {
			t.Fatalf("disable_pkce for %q = %v", setting, cfg.Server.OAuth["oidc"].DisablePKCE)
		}
		data, err := yaml.Marshal(cfg)
		if err != nil {
			t.Fatal(err)
		}
		reloaded, err := parseConfig(data)
		if err != nil {
			t.Fatal(err)
		}
		if reloaded.Server.OAuth["oidc"].DisablePKCE != wantDisabled {
			t.Fatal("disable_pkce changed after saving config")
		}
	}
	for _, value := range []string{"true", "false"} {
		t.Setenv("HISTER__SERVER__OAUTH__OIDC__DISABLE_PKCE", value)
		cfg, err := parseConfig([]byte(raw))
		if err != nil {
			t.Fatal(err)
		}
		if cfg.Server.OAuth["oidc"].DisablePKCE != (value == "true") {
			t.Fatalf("environment override %q was not applied", value)
		}
	}
}

func TestOAuthIdentityClaimConfig(t *testing.T) {
	const raw = "server:\n  oauth:\n    oidc:\n      client_id: hister\n      client_secret: secret\n      auth_url: https://provider.example/auth\n"
	for _, claim := range []string{"", "email", "sub", "https://example.com/subject"} {
		t.Run(claim, func(t *testing.T) {
			cfg, err := parseConfig([]byte(raw + "      identity_claim: '" + claim + "'\n"))
			if err != nil {
				t.Fatal(err)
			}
			if err := cfg.validateOAuth(); err != nil {
				t.Fatal(err)
			}
			data, err := yaml.Marshal(cfg)
			if err != nil {
				t.Fatal(err)
			}
			reloaded, err := parseConfig(data)
			if err != nil {
				t.Fatal(err)
			}
			if got := reloaded.Server.OAuth["oidc"].IdentityClaim; got != claim {
				t.Fatalf("identity_claim = %q, want %q", got, claim)
			}
		})
	}
	t.Run("environment overrides YAML", func(t *testing.T) {
		t.Setenv("HISTER__SERVER__OAUTH__OIDC__IDENTITY_CLAIM", "sub")
		cfg, err := parseConfig([]byte(raw + "      identity_claim: email\n"))
		if err != nil {
			t.Fatal(err)
		}
		if got := cfg.Server.OAuth["oidc"].IdentityClaim; got != "sub" {
			t.Fatalf("identity_claim = %q, want sub", got)
		}
	})
}

func TestValidateOAuthIdentityClaim(t *testing.T) {
	for _, provider := range []string{"oidc", "github", "google"} {
		for _, claim := range []string{"", "email", "sub", "https://example.com/subject", " sub", "sub ", "su b", "sub\t", "sub\n", "su\u00a0b", "sub\x00"} {
			t.Run(provider+"/"+claim, func(t *testing.T) {
				cfg := CreateDefaultConfig()
				cfg.Server.OAuth = map[string]*OAuthEntry{provider: {
					ClientID: "hister", ClientSecret: "secret", AuthURL: "https://provider.example/auth", IdentityClaim: claim,
				}}
				valid := claim == "" || provider == "oidc" && (claim == "email" || claim == "sub" || claim == "https://example.com/subject")
				err := cfg.validateOAuth()
				if valid {
					if err != nil {
						t.Fatal(err)
					}
				} else if err == nil || !strings.Contains(err.Error(), "identity_claim") {
					t.Fatalf("validation error = %v, want identity_claim error", err)
				}
			})
		}
	}
}
