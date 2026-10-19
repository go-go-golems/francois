# Stripe maximum amount and one-time Checkout

Primary source: [Supported currencies: maximum charge amounts](https://docs.stripe.com/currencies#minimum-and-maximum-charge-amounts), checked 2026-10-18 UTC. Stripe documents an eight-digit amount limit for USD, corresponding to 99,999,999 cents or 999,999.99 USD. Card-network limits can be lower.

Implementation uses Checkout `mode=payment`, fixed USD inline price data, quantity one and invoice creation. It omits recurring/subscription data. The existing stripe-go/v86 SDK and the saved real sandbox receipt verify the exact request behavior. Source API: [Create a Checkout Session](https://docs.stripe.com/api/checkout/sessions/create).
