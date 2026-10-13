output "organization_id" {
  description = "Seed into kv/apps/francois/prod/runtime as oidc-org-id."
  value       = zitadel_organization.francois.id
}

output "project_id" {
  description = "Francois ZITADEL project ID."
  value       = zitadel_project_v2.francois.id
}

output "oidc_client_id" {
  description = "Nonsecret public PKCE client ID; seed into kv/apps/francois/prod/runtime as oidc-client-id."
  value       = nonsensitive(zitadel_application_v2.francois_portal.oidc[0].client_id)
}

output "issuer" {
  description = "Value for catalog serve --oidc-issuer."
  value       = "https://${var.zitadel_domain}"
}
