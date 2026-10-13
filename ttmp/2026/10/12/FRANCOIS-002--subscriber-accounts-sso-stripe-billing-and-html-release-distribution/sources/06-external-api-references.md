# External API research references

Reviewed on 2026-10-12 for FRANCOIS-002. These are annotated links, not full webpage snapshots.

- https://zitadel.com/docs/examples/login/go — current supported Go web login with PKCE.
- https://openid.net/specs/openid-connect-core-1_0.html — issuer/subject claims and ID-token validation.
- https://docs.stripe.com/api/checkout/sessions/create — hosted subscription checkout parameters.
- https://docs.stripe.com/api/idempotent_requests — retry identity and parameter consistency.
- https://docs.stripe.com/webhooks — raw-body signatures, durable asynchronous handling, duplicates and ordering.
- https://docs.stripe.com/billing/subscriptions/webhooks — subscription and payment event integration.
- https://docs.stripe.com/api/events/types — event resource schema reference.
- https://docs.stripe.com/changelog/basil/2025-03-31/deprecate-subscription-current-period-start-and-end — period fields moved to subscription items.
- https://docs.stripe.com/customer-management/integrate-customer-portal — authenticated customer portal sessions and subscription updates.
- https://docs.aws.amazon.com/ses/latest/dg/send-email-concepts-deliverability.html — sender verification and delivery feedback.

The direct Stripe API portal-session URL returned an internal error in the browsing tool; the official portal integration page provided the equivalent endpoint and contract. No remote account or resource was mutated.
