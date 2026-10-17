APIs Core Resources V2 [Session](/docs/reference/api/session)

## Create Session

Create a new session with initial checks, metadata and challenges for further verification. A token will be returned, which is required for using the session as authentication, e.g. when authenticating an OIDC auth request or SAML request. Additionally, the session token can be used as OAuth2 access token to authenticate against the Zitadel APIs.

Required permissions:

- `session.write`

## Request Body

`application/json`

TypeScript Definitions

Use the request body type in TypeScript.

## Response Body

```
curl -X POST "https://example.com/v2/sessions" \
  -H "Content-Type: application/json" \
  -d '{}'
```

```
{

  "details": {

    "sequence": 0,

    "changeDate": "2023-01-15T01:30:15.01Z",

    "resourceOwner": "string",

    "creationDate": "2023-01-15T01:30:15.01Z"

  },

  "sessionId": "string",

  "sessionToken": "string",

  "challenges": {

    "webAuthN": {

      "publicKeyCredentialRequestOptions": {

        "property1": null,

        "property2": null

      }

    },

    "otpSms": "string",

    "otpEmail": "string"

  }

}
```

Was this page helpful?[Session API](/docs/reference/api/session)

[

Explore the Session API operation in the ZITADEL API. Learn about request parameters, response schemas, and integration details for this endpoint.

](/docs/reference/api/session)[

DeleteSession

DeleteSession: Terminate an existing session. This invalidates the session and its token. The session can no longer be used for the authentication of other resources or to authenticate against the Zitadel APIs. Y...

](/docs/reference/api/session/zitadel.session.v2.SessionService.DeleteSession)