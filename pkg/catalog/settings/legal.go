package settings

// Default texts of /privacy and /terms (Markdown). {site} becomes the site's
// host name and {contact} the legal contact address; both are editable as
// the copy keys legal.privacy and legal.terms.

const defaultPrivacy = `Last updated 16 October 2026.

## What this covers

{site} is a hosted catalog of browser applications. You sign in with an invited account, subscribe, and run applications in private instances that you can share with other members. This policy describes what the service stores about you, why, and who else processes it. It applies to every address the service is offered under, including {site}.

## Account

Sign-in runs through our own identity service (ZITADEL), either with a password or with GitHub or Google. From it we receive an account identifier, your name, your email address and whether the address is verified. With GitHub or Google we request only your basic profile and email address; we do not access repositories, contacts, files or anything else in those accounts. We also store your role and whether your account is enabled.

## Payments

Subscriptions are handled by Stripe. Card details go directly to Stripe and never reach our servers. We store the Stripe customer and subscription identifiers, the subscription status and the billing events Stripe sends us, so that access follows your subscription.

## Instances and their content

An instance is a private copy of an application. What you and the members you invite create in it — documents, shared room messages and uploaded files — is stored on our servers for that instance. Each member also has a private area that other members cannot read. Applications cannot read your account cookies or call the account site, because they run on separate addresses.

## Language-model requests

When an application asks a language model for an answer, the request and the answer pass through our server to a model provider (through the LunaRoute gateway) and back to the application. We do not store the text of requests or answers. For each request we record the model, the number of tokens and the credits used, so that allowances can be enforced. The provider processes the text under its own terms to produce the answer.

## Cookies and logs

We use two strictly necessary cookies: one for your signed-in session and one that protects the sign-in flow while it is in progress. For first-party traffic analytics we also use a random visitor cookie (90 days) and a visit cookie (30 minutes of inactivity). We record successful portal page loads, referring hostname, campaign tags, broad browser and device categories, and your account identifier when signed in. We do not store IP addresses, full browser identifiers, or full referring URLs in analytics. Do Not Track and Global Privacy Control requests are excluded. Traffic records are deleted after 90 days; model accounting records are kept separately. There are no advertising cookies. Our servers keep technical logs (time, address, requested path, errors) for operation and security.

## Why we use this data

We use it to provide the service: to sign you in, decide what you may access, run and share your instances, enforce model allowances, bill your subscription, keep the service secure and answer your questions. We do not sell your data and do not use it for advertising.

## Who processes it

The service runs on servers rented from Hetzner in Germany, with nightly backups of the database and application files in object storage. Stripe processes payments, the model provider processes model requests, and GitHub or Google take part in sign-in if you choose them. We disclose data to others only when the law requires it.

## How long we keep it

Account data is kept while your account exists. Instance content is kept until the instance is deleted. Billing records are kept as long as tax and accounting law requires. Logs and backups are deleted on a rolling basis.

## Your rights

You can ask for a copy of your data, its correction or deletion, a restriction of its use, or an export. Under the GDPR you may also object to processing and complain to a data protection authority. To make a request, use the contact below.

## Children

The service is not intended for children under 16.

## Changes

When this policy changes, the date above changes with it. Significant changes are announced to signed-in users.

## Contact

Write to {contact}.`

const defaultTerms = `Last updated 16 October 2026.

## The service

{site} is a hosted catalog of browser applications. These terms govern your use of it under every address it is offered under. By signing in you accept them.

## Accounts

Accounts are by invitation only. Keep your sign-in credentials to yourself; you are responsible for activity under your account. Tell us promptly if you believe your account has been misused.

## Subscriptions

Paid access is a subscription billed through Stripe at the price shown at checkout. It renews each period until you cancel it in the billing portal on your account page; cancellation takes effect at the end of the paid period. Payments are not refunded except where the law requires it. Model use is limited by credit allowances, which we may adjust.

## Your content

Content you create in your instances remains yours. You allow us to store, copy and process it as needed to run the service for you and the members you share an instance with. You are responsible for the content you add and for whom you share it with.

## Acceptable use

Do not use the service to break the law, to infringe others' rights, to store or distribute harmful material, to attack or overload the service, to get around access controls or allowances, or to access instances you have not been invited to.

## Applications and model output

Applications in the catalog and answers produced by language models are provided as they are. Model output can be wrong or incomplete; check it before you rely on it, and do not treat it as professional advice.

## Availability

We aim to keep the service running and back it up nightly, but we do not guarantee uninterrupted availability or that data cannot be lost. Keep your own copies of anything important. We may change or discontinue features, and will give notice before discontinuing the service.

## Suspension and ending

You can stop using the service at any time and ask for your account to be deleted. We may suspend or close accounts that break these terms.

## Liability

To the extent the law allows, the service is provided without warranties, and our liability is limited to the amount you paid in the twelve months before the claim. Nothing in these terms limits liability that cannot be limited by law.

## Changes

When these terms change, the date above changes with it. Continuing to use the service after a change means you accept the new terms.

## Contact

Write to {contact}.`
