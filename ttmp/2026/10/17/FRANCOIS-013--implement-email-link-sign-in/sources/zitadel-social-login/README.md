# ZITADEL social sign-in sources

Checked on 2026-10-17 against the application’s deployed ZITADEL v4.16.1 and existing Terraform provider v3.3.0.

- [Official reserved scopes](https://zitadel.com/docs/apis/openidoauth/scopes): `urn:zitadel:iam:org:idp:id:{idp_id}` selects an external provider directly. Keep the existing organization scope so the organization’s login policy and providers are selected together.
- [Official Google integration](https://zitadel.com/docs/guides/integrate/identity-providers/google): account creation permission and automatic creation are distinct settings; an organization’s external login policy must include the provider.
- Existing Terraform resources: `zitadel/francois/envs/prod/main.tf` in wesen/terraform. The reviewed social-signup plan changes only the Francois login policy and its two provider templates in place; credentials and existing linking rules are retained.
- The application still uses its existing OIDC state, browser binding, nonce, S256 PKCE, token verification and issuer/subject-based local identity. Provider names from the browser are allowlisted; provider IDs are server configuration.

The original ZITADEL Session API research under ../zitadel-email-login remains applicable to email links. Social sign-in does not itself grant a paid subscription or a staff role.
