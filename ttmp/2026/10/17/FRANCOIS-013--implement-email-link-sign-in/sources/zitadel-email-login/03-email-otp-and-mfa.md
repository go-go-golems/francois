## Multi-Factor Authentication (MFA) in a Custom Login UI

Multi-factor authentication (MFA) is a multi-step account authentication which requires to user to enter more than only the password. It is highly recommended to use MFA or [Passkeys](./passkey) to make your user accounts more secure.

ZITADEL supports different Methods:

- Time-based one time password (TOTP), which are Authenticator apps like Google/Microsoft Authenticator, Authy, etc
- OTP sent as SMS
- OTP sent as Email
- Universal Second Factor (U2F), which is authentication with your device like Windows Hello, Apple FaceID, Fingerprint, FIDO2 keys, Yubikey, etc.

## TOTP Registration

### Flow

![](/docs/_next/image?url=%2Fdocs%2F_next%2Fstatic%2Fimmutable%2Fmedia%2Fregister-totp-flow.43cup3lwpuh3s.png&w=3840&q=75)

### List the Possible Methods

Your user has successfully authenticated, and now you ask them if they want to set up MFA to have a more secure account. When the user starts the setup, first you want to show them the possible methods. You can either list it implicitly or call the settings service from ZITADEL to get what is configured on the login settings.

More detailed information about the API: [Get Login Settings Documentation](/docs/reference/api/settings/zitadel.settings.v2.SettingsService.GetLoginSettings)

Request Example:

Response Example: The relevant part for the list is the second factor and multifactor list.

### Start TOTP Registration

The user has selected to setup TOTP. To show the user the QR to register TOTP with his Authenticator App like Google/Microsoft Authenticator or Authy you have to start the registration on the ZITADEL API. Generate the QR Code with the URI from the response. For users that do not have a QR Code reader make sure to also show the secret, to enable manual setup.

More detailed information about the API: [Start TOTP Registration Documentation](/docs/reference/api/user/zitadel.user.v2.UserService.RegisterTOTP)

Request Example:

Response Example:

```
{

  "details": {

    "sequence": "2",

    "changeDate": "2023-06-28",

    "resourceOwner": "69629023906488334"

  },

  "uri": "otpauth://totp/ZITADEL:minni-mouse@mouse.com?algorithm=SHA1&digits=6&issuer=ZITADEL&period=30&secret=TJOPWSDYILLHXFV4MLKNNJOWFG7VSDCK",

  "secret": "TJOPWSDYILLHXFV4MLKNNJOWFG7VSDCK"

}
```

### Verify TOTP Registration

When the user has added the account to the authenticator app, the code from the App has to be entered to finish the registration. This code has to be sent to the verify endpoint in ZITADEL.

More detailed information about the API: [Verify TOTP Documentation](/docs/reference/api/user/zitadel.user.v2.UserService.VerifyTOTPRegistration)

Request Example:

## TOTP Authentication

### Flow

![](/docs/_next/image?url=%2Fdocs%2F_next%2Fstatic%2Fimmutable%2Fmedia%2Fauthenticate-totp-flow.3aygopx6pxkps.png&w=3840&q=75)

### Check User

To be able to check the TOTP you need a session with a checked user. This can either happen before the TOTP check or at the same time. In this example we do two separate requests. So the first step is to create a new Sessions.

More detailed information about the API: [Create new session Documentation](/docs/reference/api/session/zitadel.session.v2.SessionService.CreateSession)

Example Request

Example Response

```
{

    "details": {

        "sequence": "580",

        "changeDate": "2023-06-14T05:32:39.007096Z",

        "resourceOwner": "163840776835432705"

    },

    "sessionId": "218480890961985793",

    "sessionToken": "yMDi6uVPJAcphbbz0LaxC07ihWkNTe7m0Xqch8SzfM5Cz3HSIQIDZ65x1f5Qal0jxz0MEyo-_zYcUg"

}
```

### Check TOTP

Now you can show the code field to the user, where the code needs to be entered from the Authenticator App. With that code you have to update the existing session with a totp check.

More detailed information about the API: [Update session Documentation](/docs/reference/api/session/zitadel.session.v2.SessionService.SetSession)

Example Request

## SMS Code Registration

### Flow

![](/docs/_next/image?url=%2Fdocs%2F_next%2Fstatic%2Fimmutable%2Fmedia%2Fregister-phone-otp-flow.392ufjps7fmln.png&w=3840&q=75)

### List the Possible Methods

Your user has successfully authenticated, and now you ask them if they want to set up MFA to have a more secure account. When the user starts the setup, first you want to show them the possible methods. You can either list it implicitly or call the settings service from ZITADEL to get what is configured on the login settings.

More detailed information about the API: [Get Login Settings Documentation](/docs/reference/api/settings/zitadel.settings.v2.SettingsService.GetLoginSettings)

Request Example:

Response Example: The relevant part for the list is the second factor and multifactor list.

### Add Phone Number

When the user has decided to register the phone number to get a code as a second factor, the first step is to add a verified phone number to the user. If the user already has a verified phone number you can skip this step.

When adding a new phone number, you can choose if you want ZITADEL to send the verification code to the number, or if you want to send it by yourself. If ZITADEL should do it, make sure that you have registered an [SMS Provider](/docs/guides/manage/console/default-settings#sms) and send an empty sendCode object in the request. With an empty returnCode object in the request, ZITADEL will not send the code, but return it in the response.

If you don't want the user to verify the phone number, you can also create it directly as verified, by sending the isVerified attribute.

More detailed information about the API: [Add phone](/docs/reference/api/user/zitadel.user.v2.UserService.SetPhone)

Example Request:

### Verify Phone Number

The next step is to show a screen, so the user is able to enter the code for verifying the phone number. Send a verify phone request with the code in the body.

More detailed information about the API: [Verify phone](/docs/reference/api/user/zitadel.user.v2.UserService.VerifyPhone)

Example Request:

### Add OTP SMS to the user

Now that the user has a verified phone number you can enable OTP SMS on the user.

More detailed information about the API: [Add OTP SMS for a user](/docs/reference/api/user/zitadel.user.v2.UserService.AddOTPSMS)

Example Request:

## SMS Code Authentication

### Flow

![](/docs/_next/image?url=%2Fdocs%2F_next%2Fstatic%2Fimmutable%2Fmedia%2Fauthenticate-phone-otp-flow.1-t607-a3r2zd.png&w=3840&q=75)

### Check User

To be able to check the SMS Code you need a session with a checked user. When creating the session you can already start the sms challenge, this will only be executed if the user check was successful. You can tell the challenge, if the code should be returned (returnCode: true) or if ZITADEL should send it (returnCode: false).

More detailed information about the API: [Create new session Documentation](/docs/reference/api/session/zitadel.session.v2.SessionService.CreateSession)

Example Request

### Check SMS Code

In the next step you should prompt the user to enter the SMS verification code in the provided field. The update session request has a check otpSMS where you should send the code, the user has entered.

Example Request

## Email Code Registration

### Flow

![](/docs/_next/image?url=%2Fdocs%2F_next%2Fstatic%2Fimmutable%2Fmedia%2Fregister-email-otp-flow.11alxtk25o2qb.png&w=3840&q=75)

### List the Possible Methods

Your user has successfully authenticated, and now you ask them if they want to set up MFA to have a more secure account. When the user starts the setup, first you want to show them the possible methods. You can either list it implicitly or call the settings service from ZITADEL to get what is configured on the login settings.

More detailed information about the API: [Get Login Settings Documentation](/docs/reference/api/settings/zitadel.settings.v2.SettingsService.GetLoginSettings)

Request Example:

Response Example: The relevant part for the list is the second factor and multifactor list.

### Verified Email

As ZITADEL required all users to have a verified email address, you do not need to add a new email and verify it for setting up the second factor. For the Email second factor the already verified email address will be taken.

### Add OTP Email to the user

As the user already has a verified E-Mail address you can enable E-Mail OTP on the user.

More detailed information about the API: [Add OTP Email for a user](/docs/reference/api/user/zitadel.user.v2.UserService.AddOTPEmail)

Example Request:

## Email Code Authentication

### Flow

![](/docs/_next/image?url=%2Fdocs%2F_next%2Fstatic%2Fimmutable%2Fmedia%2Fauthenticate-email-otp-flow.3-qmq56oyj1vs.png&w=3840&q=75)

### Check User

To be able to check the Email Code you need a session with a checked user. When creating the session you can already start the sms challenge, this will only be executed if the user check was successful. You can tell the challenge, if the code should be returned (returnCode: true) or if ZITADEL should send it (returnCode: false).

More detailed information about the API: [Create new session Documentation](/docs/reference/api/session/zitadel.session.v2.SessionService.CreateSession)

Example Request

### Check Email Code

Next, display a code field for the user to enter the verification code they received via email. The update session request has a check otpEmail where you should send the code, the user has entered.

Example Request

## U2F Registration

### Flow

![](/docs/_next/image?url=%2Fdocs%2F_next%2Fstatic%2Fimmutable%2Fmedia%2Fregister-u2f-flow.43s8y-ijqhyb-.png&w=3840&q=75)

### List the Possible Methods

Your user has successfully authenticated, and now you ask them if they want to set up MFA to have a more secure account. When the user starts the setup, first you want to show them the possible methods. You can either list it implicitly or call the settings service from ZITADEL to get what is configured on the login settings.

More detailed information about the API: [Get Login Settings Documentation](/docs/reference/api/settings/zitadel.settings.v2.SettingsService.GetLoginSettings)

Request Example:

Response Example: The relevant part for the list is the second factor and multifactor list.

### Start U2F Registration

The user has selected to setup Universal Second Factor (U2F). To be able to authenticate in the browser you have to start the u2f registration within ZITADEL.

More detailed information about the API: [Start U2F Registration Documentation](/docs/reference/api/user/zitadel.user.v2.UserService.RegisterU2F)

Request Example:

Response Example:

```
{

  "details": {

    "sequence": "2",

    "changeDate": "2023-07-03",

    "resourceOwner": "69629023906488334"

  },

  "u2fId": "163840776835432705",

  "publicKeyCredentialCreationOptions": {

    "publicKey": {

      "attestation": "none",

      "authenticatorSelection": {

        "userVerification": "required"

      },

      "challenge": "XaMYwWOZ5hj6pwtwJJlpcI-ExkO5TxevBMG4R8DoKQQ",

      "excludeCredentials": [

        {

          "id": "tVp1QfYhT8DkyEHVrv7blnpAo2YJzbZgZNBf7zPs6CI",

          "type": "public-key"

        }

      ],

      "pubKeyCredParams": [

        {

          "alg": -7,

          "type": "public-key"

        }

      ],

      "rp": {

        "id": "localhost",

        "name": "ZITADEL"

      },

      "timeout": 300000,

      "user": {

        "displayName": "Minie Mouse",

        "id": "MjE1NTk4MDAwNDY0OTk4OTQw",

        "name": "minie-mouse"

      }

    }

  }

}
```

Now that you have started the registration within ZITADEL, you have to register the credentials in the browser. This requires a call to the browser api and looks something like the following. Make sure to send the public key credential creation options you got in the previous request from ZITADEL.

For more information about WebAuthN and registering credential flow, read the following guide:[Registering a WebAuthN Credentials](https://webauthn.guide/#registration)

### Verify U2F Registration

In the next request you have to verify the U2F within ZITADEL. Include the public key credential you got from the browser in your request. You can give the U2F a name, which makes it easier for the user to identify the registered authentication methods. Example: Google Pixel, iCloud Keychain, Yubikey, etc

More detailed information about the API: [Verify U2F Documentation](/docs/reference/api/user/zitadel.user.v2.UserService.VerifyU2FRegistration)

Example Request:

You have successfully registered a new U2F to the user.

## U2F Authentication

### Flow

![](/docs/_next/image?url=%2Fdocs%2F_next%2Fstatic%2Fimmutable%2Fmedia%2Fauthenticate-u2f-flow.3n419zrp8-vja.png&w=3840&q=75)

### Check User

To be able to check the Universal Second Factor (U2F) you need a user check and a webAuthN challenge. In the create session request you can check for the user and directly initiate the webAuthN challenge.

For U2F you can choose between "USER\_VERIFICATION\_REQUIREMENT\_PREFERRED" and "USER\_VERIFICATION\_REQUIREMENT\_DISCOURAGED" for the challenge. Best practice is using discouraged, as this doesn't require the user to enter a PIN. With `preferred` the user might be prompted for the PIN, but it is not necessary.

More detailed information about the API: [Create new session Documentation](/docs/reference/api/session/zitadel.session.v2.SessionService.CreateSession)

Example Request

Example Response

```
{

    "details": {

        "sequence": "580",

        "changeDate": "2023-06-14T05:32:39.007096Z",

        "resourceOwner": "163840776835432705"

    },

    "sessionId": "218480890961985793",

    "sessionToken": "yMDi6uVPJAcphbbz0LaxC07ihWkNTe7m0Xqch8SzfM5Cz3HSIQIDZ65x1f5Qal0jxz0MEyo-_zYcUg"

}
```

### Signin in Browser

After starting the WebAuthN authentication on the side of ZITADEL you have to challenge the browser. To do this you need to call the browser API to get the credentials. Make sure to send the public key credential request options you got from ZITADEL.

Read the [WebAuthN Guide](https://webauthn.guide/#authentication) for more information about "Authenticating with a WebAuthN Credential".

### Update Session with WebAuthN

Now that you have successfully authenticated in the browser, you can update the session of the user. Fill the webAuthN checks with the credential assertion data you get from the browser.

More detailed information about the API: [Update Session Documentation](/docs/reference/api/session/zitadel.session.v2.SessionService.SetSession)

Example Request:

Was this page helpful?[Passkeys](/docs/guides/integrate/login-ui/passkey)

[

Register and verify a Passkey with ZITADEL to enable phishing-resistant authentication using the FIDO Standard protocol

](/docs/guides/integrate/login-ui/passkey)[

Select Account in a Custom Login UI

Implement custom account selection logic using React components to handle user authentication flows with multiple identity providers

](/docs/guides/integrate/login-ui/select-account)