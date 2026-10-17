package catalog

import (
	"context"
	"net/http"

	"github.com/go-go-golems/francois/pkg/catalog/store"
)

func (s *Server) issueEmailRegistration(ctx context.Context, r *http.Request, email, binding string) error {
	provider := s.Config.EmailLogin
	token := store.RandomHex(32)
	f, old, err := s.Store.ReplaceEmailRegistration(ctx, provider.Issuer(), email, token, store.Hash(binding), s.site(r).PortalOrigin(), safeReturn(r.PostForm.Get("return")))
	if err != nil {
		return err
	}
	if old != "" {
		_ = provider.DeleteSession(ctx, old)
	}
	template := s.site(r).PortalOrigin() + "/auth/email/verify?flow=" + token + "&code={{.Code}}"
	session, err := provider.SendRegistration(ctx, f.Subject, email, template)
	if err != nil {
		return err
	}
	attached, err := s.Store.AttachEmailRegistration(ctx, token, session)
	if err != nil || !attached {
		_ = provider.DeleteSession(ctx, session)
	}
	return err
}
