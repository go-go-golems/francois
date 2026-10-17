[Skip to content](#start-of-content)

Discussion options

| People have too many passwords and a lot of people don't use a password manager.  Besides supporting Federated identity management, it would be very helpful to have support for magic links.  Example: [https://www.youtube.com/watch?v=hMqxo68ZJlw](https://www.youtube.com/watch?v=hMqxo68ZJlw)  1. Would it be possible for zitadel to support this style of magic links? 2. Alternately a magic token or phrase can work too 3. If this sounds like a good idea, how quickly can we add support for this style of magic links:) |
| --- |

You must be logged in to vote Comment options

| Hi [@gc-ss](https://github.com/gc-ss)  I totally agree on the problem of managing passwords. Actually people without password managers tend to have only a few passwords, which they use nn a lot of websites. This is even a bigger problem.  That's why we are advocate passwordless based on FIDO2 / CTAP2. So people don't have to remember (a potentially weak) password, but can rely on a cryptographically secure mechanism.  Personally, I'd first encourage users to go passwordless using FIDO2. Nearly everyone or at least the majority of people already have a FIDO2 enabled device (any Apple device having TouchID / FaceID, Android devices, Windows Hello,...). And there are USB keys as well like SoloKeys, Yubikey,...  But magic links could serve as a second option for a passwordless flow. We would just have to discuss the use cases. It might be a case where people are not allowed to use their (personal) phone at work and don't have a FIDO key.   Who is able to generate a link? Is downgrading possible? By that i mean: If a users registered for FIDO2 passwordless (or FIDO1 / U2F as second factor), creating and using a magic link would weaken the security of this account. |
| --- |

You must be logged in to vote

2 replies

Comment options

| > But magic links could serve as a second option for a passwordless flow. We would just have to discuss the use cases. It might be a case where people are not allowed to use their (personal) phone at work and don't have a FIDO key  Right - you know more about this market than I do but from my small sample size - very few end users have a FIDO key - unless zitadel supports software OTP like Google/MS Auth - does it?  > Who is able to generate a link?  zitadel does based on the option set by the admin for the user. See: [https://www.youtube.com/watch?v=hMqxo68ZJlw](https://www.youtube.com/watch?v=hMqxo68ZJlw)  > If a users registered for FIDO2 passwordless (or FIDO1 / U2F as second factor), creating and using a magic link would weaken the security of this account  True. If user has a registered FIDO key, we could encourage the user not to use the link option through a message or toaster notification. Later when we have more time, we can write more advanced logic. |
| --- |

Comment options

| Just a sidenote  > Right - you know more about this market than I do but from my small sample size - very few end users have a FIDO key - unless zitadel supports software OTP like Google/MS Auth - does it?  Well, Windows Hello, Apple and Android Support this as well. |
| --- |

Comment options

| I worry about magic links due to my experience with Discord's magic links.  For those unfamiliar, Discord has a system where if you're logged in on your phone and you wish to log in on desktop, you can scan a QR code which is simply a magic link. Their implementation bypasses MFA and is generally very insecure.  The worst part about it is that it's very easy to spoof. Recently, there has been a scam involving these QR codes pretending to be some sort of captcha:  [![Fake captcha](https://camo.githubusercontent.com/91c593eb5c585493b83d17ad96cb2d673f57f595ea898f7ea57eb7fc378369c3/68747470733a2f2f63646e2e646973636f72646170702e636f6d2f6174746163686d656e74732f3739303239383831363831373030343538382f3937333735373437373336313434323836362f756e6b6e6f776e2e706e67)](https://camo.githubusercontent.com/91c593eb5c585493b83d17ad96cb2d673f57f595ea898f7ea57eb7fc378369c3/68747470733a2f2f63646e2e646973636f72646170702e636f6d2f6174746163686d656e74732f3739303239383831363831373030343538382f3937333735373437373336313434323836362f756e6b6e6f776e2e706e67)  Scanning this QR code sends your logon token to the malicious user, allowing them to turn your account into a bot account. For those who are technical, this scheme fortunately has some very similar attributes to phishing scams, so they should be relatively easy to spot. However, there are a lot of people who simply follow directions without thinking, and like a phishing scam, this is their primary audience.  > Personally, I'd first encourage users to go passwordless using FIDO2. Nearly everyone or at least the majority of people already have a FIDO2 enabled device (any Apple device having TouchID / FaceID, Android devices, Windows Hello,...). And there are USB keys as well like SoloKeys, Yubikey,...  I would suggest that this a sufficient solution.   For those who do not have a FIDO2 device, one could follow Microsoft's model and, assuming a user has logged in to a specific system in the past (say a website and they have the correct cookie), then auth could be typing a username and responding to a push notification or entering the TOTP code. This would not be a magic link, but it would give you the passwordless authentication method you are looking for. |
| --- |

You must be logged in to vote

1 reply

Comment options

| > This would not be a magic link, but it would give you the passwordless authentication method you are looking for.  Your concern is well understood. Security depends on the usecase.  If my site shows cat pictures to paying members, and I don't want them to worry about passwords, a magic link that gets emailed to them is good enough.  I agree that a magic link isn't the most secure solution for people who simply follow directions without thinking, but if my usecase is to allow people who simply follow directions without thinking an easy way to login without thinking much, magic links are a great solution.  Since zitadel is an authz solution, supporting magic links is a core feature.  Users of zitadel decide if magic links are a good use for their own usecase. I wouldn't expect a bank to use magic links to allow access to funds transfer but perhaps the same bank can use magic links to send people who have forgotten their bank password to customer support. |
| --- |

Comment options

| Is there a way we can implement this passwordless login using a pre-authenticated email link.  Without prompting the user to register the device using a security key or Android phone |
| --- |

You must be logged in to vote

3 replies

Comment options

| Actually that's already implemented and there are two options for this:  - Send an email with a link to pair and onboard a FIDO2 compatible device. - Request a QR code to pair and onboard a FIDO2 compatible device.  ## Management API  - [AddPasswordlessRegistration](https://docs.zitadel.com/docs/apis/proto/management#addpasswordlessregistration) - [SendPasswordlessRegistration](https://docs.zitadel.com/docs/apis/proto/management#sendpasswordlessregistration)  ## User Self-Service  [![image](https://user-images.githubusercontent.com/9879976/176517330-7c86c47e-75ef-40a6-828a-243e4160740e.png)](https://user-images.githubusercontent.com/9879976/176517330-7c86c47e-75ef-40a6-828a-243e4160740e.png) |
| --- |

Comment options

| I have tried this feature but I am referring to a simple authentication mechanism that does not require a FIDO2 compatible device as many people generally don't use mechanism  Mechanism I am looking for Ace to buy pasta requirement of linking or pairing on board FIDO2 compatible device and just authenticate the browser directly maybe a OTP or pre authenticated link. |
| --- |

Comment options

| Ok, so you think "classic" magic links 😁 |
| --- |

Comment options

| Based on feedback from community members, I think soft-onboarding users with Magic Links could be a valuable use case.  Example for soft-onboarding:   App that let's B2B customers invite end-users to RSVP for their event. The customer uploads a list of users to invite. Users receive a personalized (magic) link to RSVP. They are also offered to create an account (social login, usr/pwd, passkeys...) and get access to the platform eg, to manage their profile or bookings. |
| --- |

You must be logged in to vote

0 replies

Comment options

| I'd say magic links should be alternative to password recovery. Ie do not bypass 2fa.   I personally do no remember password for booking.com, so when logging in you typpe username -> next -> type password/send magic link -> send magic link -> email with link(super long link tho) -> logged in |
| --- |

You must be logged in to vote

3 replies

Comment options

| May I ask why passkeys would not work for you in that case? Do you think magic links would be easier to understand? |
| --- |

Comment options

| Well magic links are a lot simpler and also passkeys are not available everywhere.  1. My PC cant be used directly for passkeys, I dont have FIDO and having to reach for a phone is nconveniece 2. Just checked, even my android tablet is incompatible |
| --- |

Comment options

| Sure, compatibility is an issue then. Thanks for clarifying! |
| --- |

Comment options

| I'm working on a tool for people for whom technology is not really easy (elderly people for exemple) and the FIDO2 solution is way too complex for them. In my case I prefer magic links, event if I know the down sides from a security point of view. Anyway they do not have access to sensitive information, I prefer for them the smoothest experience rather than rock-solid security.  Some of our users are resetting their password for every login attempt... |
| --- |

You must be logged in to vote

0 replies

Comment options

| I'm a noob when it comes to auth, but would it be possible to implement magic links with [token exchange](https://zitadel.com/docs/guides/integrate/token-exchange)? |
| --- |

You must be logged in to vote

5 replies

Comment options

| Hm, do you mind sharing what you are looking for? |
| --- |

Comment options

| I'm looking for a way to log in users with magic links.  Magic link is a process where the application that uses a service account generates a user token, sends it over to the user and they use it to access the protected resource. The first step sounds like impersonation described in the link above. |
| --- |

Comment options

| Got it, you could use the session-api to authenticate a user and then your app could request a token for that user with the impersonation. So the answer is yes, but it might need some glue code on your end.  I think we would also look into a PR 😁 |
| --- |

Comment options

| I might write a POC in pyhton or TS when time allows, but I'm not up for diving into golang and Zitadel codebase to prepare a PR.   Perhaps there'll be another volunteer, now that the key exchange is available. |
| --- |

Comment options

| Sure, that sounds great, you might be able to reuse some stuff from our TS repo [https://github.com/zitadel/typescript](https://github.com/zitadel/typescript) |
| --- |

Comment options

| Hi! What's is status of supporting magic links in zitadel? |
| --- |

You must be logged in to vote

2 replies

Comment options

| hey [@melezhik](https://github.com/melezhik)   We have not built yet the functionality in our hosted login.   However, if you would like to have the possibility of MagicLink, you can use our session API and build the login UI yourself.   API Docs: [https://zitadel.com/docs/apis/resources/session\_service\_v2](https://zitadel.com/docs/apis/resources/session_service_v2)   Build your own login: [https://zitadel.com/docs/guides/integrate/login-ui](https://zitadel.com/docs/guides/integrate/login-ui) |
| --- |

Comment options

| [@rajatcing](https://github.com/rajatcing), hi, could you describe the workflow for login via one-time codes sent to email or phone? I've looked into the session API and don't fully understand how it helps. |
| --- |

Comment options

| Can confirm, it's possible to make magic links in Zitadel using the above recommendations^  All the machinery is there, just needs a bit of investigation / work. Was able to get a solution working pretty quickly 👍 |
| --- |

You must be logged in to vote

0 replies