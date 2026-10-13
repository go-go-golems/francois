provider "vault" {}

ephemeral "vault_kv_secret_v2" "zitadel_terraform" {
  mount = "kv"
  name  = "apps/zitadel/prod/terraform"
}

provider "zitadel" {
  domain       = var.zitadel_domain
  port         = "443"
  insecure     = false
  access_token = ephemeral.vault_kv_secret_v2.zitadel_terraform.data["access_token"]
}

# Francois subscribers live in their own organization. Francois sends
# urn:zitadel:iam:org:id:<id> so login and self-registration use this
# organization's policy and new users are created here.
resource "zitadel_organization" "francois" {
  name = "Francois"
}

resource "zitadel_project_v2" "francois" {
  org_id = zitadel_organization.francois.id
  name   = "francois"

  # Francois owns roles, entitlement and sharing in PostgreSQL. Any verified
  # user of this organization may authenticate to the portal client.
  project_role_assertion = false
  project_role_check     = false
  has_project_check      = false
}

resource "zitadel_application_v2" "francois_portal" {
  org_id     = zitadel_organization.francois.id
  project_id = zitadel_project_v2.francois.id
  name       = "francois-portal"

  # Authorization Code + S256 PKCE public client, as in the Compose stack
  # (deploy/compose/bin/catalog-setup) but with dev_mode off.
  oidc {
    redirect_uris               = ["${var.public_url}/auth/callback"]
    post_logout_redirect_uris   = ["${var.public_url}/"]
    response_types              = ["OIDC_RESPONSE_TYPE_CODE"]
    grant_types                 = ["OIDC_GRANT_TYPE_AUTHORIZATION_CODE"]
    app_type                    = "OIDC_APP_TYPE_WEB"
    auth_method_type            = "OIDC_AUTH_METHOD_TYPE_NONE"
    version                     = "OIDC_VERSION_1_0"
    dev_mode                    = false
    access_token_type           = "OIDC_TOKEN_TYPE_BEARER"
    id_token_userinfo_assertion = true
    access_token_role_assertion = false
    id_token_role_assertion     = false
  }
}

resource "zitadel_login_policy" "francois" {
  org_id                        = zitadel_organization.francois.id
  user_login                    = true
  allow_register                = var.allow_register
  allow_external_idp            = false
  force_mfa                     = false
  force_mfa_local_only          = false
  passwordless_type             = "PASSWORDLESS_TYPE_ALLOWED"
  hide_password_reset           = false
  password_check_lifetime       = "240h0m0s"
  external_login_check_lifetime = "240h0m0s"
  multi_factor_check_lifetime   = "24h0m0s"
  mfa_init_skip_lifetime        = "720h0m0s"
  second_factor_check_lifetime  = "24h0m0s"
  ignore_unknown_usernames      = true
  default_redirect_uri          = "${var.public_url}/"
  allow_domain_discovery        = false
  disable_login_with_email      = false
  disable_login_with_phone      = true
}
