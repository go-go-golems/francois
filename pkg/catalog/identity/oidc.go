// Package identity wraps the OIDC relying party used for subscriber login.
// It performs discovery lazily so the portal stays up (and existing sessions
// keep working) while the identity provider is unavailable.
package identity

import (
	"context"
	"sync"
	"time"

	"github.com/go-go-golems/francois/pkg/catalog/store"
	"github.com/pkg/errors"
	"github.com/zitadel/oidc/v3/pkg/client/rp"
	"github.com/zitadel/oidc/v3/pkg/oidc"
	"golang.org/x/oauth2"
)

type Config struct {
	Issuer       string
	ClientID     string
	ClientSecret string
	RedirectURL  string
}

type Provider struct {
	cfg Config
	mu  sync.Mutex
	rp  rp.RelyingParty
}

type nonceKey struct{}

func New(cfg Config) *Provider { return &Provider{cfg: cfg} }

func (p *Provider) Configured() bool { return p.cfg.Issuer != "" && p.cfg.ClientID != "" }

func (p *Provider) Issuer() string { return p.cfg.Issuer }

func (p *Provider) party(ctx context.Context) (rp.RelyingParty, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.rp != nil {
		return p.rp, nil
	}
	if !p.Configured() {
		return nil, errors.New("OIDC is not configured")
	}
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	party, err := rp.NewRelyingPartyOIDC(ctx, p.cfg.Issuer, p.cfg.ClientID, p.cfg.ClientSecret, p.cfg.RedirectURL,
		[]string{oidc.ScopeOpenID, oidc.ScopeProfile, oidc.ScopeEmail},
		rp.WithVerifierOpts(rp.WithNonce(func(ctx context.Context) string {
			n, _ := ctx.Value(nonceKey{}).(string)
			return n
		}), rp.WithIssuedAtOffset(5*time.Second)),
		rp.WithSigningAlgsFromDiscovery(),
	)
	if err != nil {
		return nil, errors.Wrap(err, "discover OIDC issuer")
	}
	p.rp = party
	return party, nil
}

// AuthURL builds the authorization redirect with S256 PKCE and a nonce.
func (p *Provider) AuthURL(ctx context.Context, state, nonce, verifier string, prompt string) (string, error) {
	party, err := p.party(ctx)
	if err != nil {
		return "", err
	}
	opts := []rp.AuthURLOpt{rp.WithCodeChallenge(oidc.NewSHACodeChallenge(verifier)), withParam("nonce", nonce)}
	if prompt != "" {
		opts = append(opts, rp.WithPrompt(prompt))
	}
	u := rp.AuthURL(state, party, opts...)
	return u, nil
}

// Exchange redeems the code with the PKCE verifier and validates the ID token
// (issuer, audience, signature, expiry and nonce) through the library.
func (p *Provider) Exchange(ctx context.Context, code, verifier, nonce string) (store.Identity, error) {
	party, err := p.party(ctx)
	if err != nil {
		return store.Identity{}, err
	}
	ctx = context.WithValue(ctx, nonceKey{}, nonce)
	tokens, err := rp.CodeExchange[*oidc.IDTokenClaims](ctx, code, party, rp.WithCodeVerifier(verifier))
	if err != nil {
		return store.Identity{}, errors.Wrap(err, "exchange authorization code")
	}
	c := tokens.IDTokenClaims
	id := store.Identity{Issuer: c.Issuer, Subject: c.Subject, Email: c.Email, EmailVerified: bool(c.EmailVerified), Name: c.Name}
	if id.Name == "" || id.Email == "" {
		// ZITADEL may omit profile claims from the ID token; ask userinfo.
		info, uerr := rp.Userinfo[*oidc.UserInfo](ctx, tokens.AccessToken, tokens.TokenType, c.Subject, party)
		if uerr == nil {
			if id.Email == "" {
				id.Email, id.EmailVerified = info.Email, bool(info.EmailVerified)
			}
			if id.Name == "" {
				id.Name = info.Name
			}
		}
	}
	if id.Subject == "" || id.Issuer != p.cfg.Issuer {
		return store.Identity{}, errors.New("identity token has an unexpected issuer or subject")
	}
	return id, nil
}

func withParam(key, value string) rp.AuthURLOpt {
	return func() []oauth2.AuthCodeOption { return []oauth2.AuthCodeOption{oauth2.SetAuthURLParam(key, value)} }
}
