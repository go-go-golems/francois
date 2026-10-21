package cmds

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/go-go-golems/francois/pkg/catalog/client"
	"github.com/go-go-golems/francois/pkg/catalog/credentials"
	"github.com/go-go-golems/francois/pkg/catalog/store"
	"github.com/go-go-golems/glazed/pkg/cli"
	glazed "github.com/go-go-golems/glazed/pkg/cmds"
	"github.com/go-go-golems/glazed/pkg/cmds/values"
	"github.com/go-go-golems/glazed/pkg/middlewares"
	"github.com/go-go-golems/glazed/pkg/types"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"
	"golang.org/x/term"
)

// The auth commands log in to a hosted catalog and keep the personal API
// token in the credentials file (see credentials.Path), which every API
// command, releases push and francois mcp then use.

func authCommands() (*cobra.Command, error) {
	g := &cobra.Command{Use: "auth", Short: "Log in to a hosted catalog and manage the stored API token"}
	for _, c := range []glazed.Command{
		&authLogin{glazed.NewCommandDescription("login",
			glazed.WithShort("Log in and store a personal API token for a server"),
			glazed.WithLong("With --with-token, reads an existing token from stdin (or a hidden prompt), checks it with the server and stores it. "+
				"The token is saved per server in the credentials file ($FRANCOIS_CONFIG_DIR, $XDG_CONFIG_HOME/francois or ~/.config/francois, mode 0600). "+
				"FRANCOIS_TOKEN and --token-file still take precedence over stored logins."),
			glazed.WithFlags(
				flag("server", "Portal origin, e.g. https://slop.boutique (default: $FRANCOIS_SERVER, then the default stored login)"),
				flag("host", "Host header override, for tunnels; stored with the login"),
				boolean("with-token", "Read an existing token from stdin instead of approving a new one in the browser"),
				boolean("no-default", "Do not make this server the default"),
			))},
		&authStatus{glazed.NewCommandDescription("status",
			glazed.WithShort("Show stored logins and which token commands use"),
			glazed.WithFlags(
				flag("server", "Only this server"),
				boolean("check", "Ask each server whether its token is still valid"),
			))},
		&authToken{glazed.NewCommandDescription("token",
			glazed.WithShort("Print the token commands would use, e.g. export FRANCOIS_TOKEN=$(francois catalog auth token)"),
			glazed.WithFlags(
				flag("server", "Portal origin (default: $FRANCOIS_SERVER, then the default stored login)"),
				boolean("yes", "Print even when stdout is a terminal"),
			))},
		&authLogout{glazed.NewCommandDescription("logout",
			glazed.WithShort("Forget the stored token for a server"),
			glazed.WithFlags(
				flag("server", "Portal origin (default: $FRANCOIS_SERVER, then the default stored login)"),
				boolean("all", "Log out of every stored server"),
			))},
	} {
		cmd, err := cli.BuildCobraCommandFromCommand(c)
		if err != nil {
			return nil, errors.Wrap(err, "auth")
		}
		g.AddCommand(cmd)
	}
	return g, nil
}

type authLogin struct{ *glazed.CommandDescription }

var _ glazed.GlazeCommand = (*authLogin)(nil)

func (c *authLogin) RunIntoGlazeProcessor(ctx context.Context, v *values.Values, out middlewares.Processor) error {
	a := in{v}
	srv, err := credentials.ResolveServer(a.S("server"))
	if err != nil {
		return err
	}
	if !a.B("with-token") {
		return errors.New("browser login is not available yet; pass --with-token")
	}
	token, err := readToken(os.Stdin)
	if err != nil {
		return err
	}
	me, err := client.New(srv.Server, token, hostOption(a.S("host"))...).Me(ctx)
	if err != nil {
		return errors.Wrap(loginHint(err), "check the token")
	}
	if me.Token == nil {
		return errors.New("the server did not describe the token")
	}
	return saveLogin(ctx, out, srv.Server, entryFor(token, me.User, *me.Token, a.S("host")), !a.B("no-default"))
}

// readToken reads a token from a hidden prompt on a terminal, else from r.
func readToken(r io.Reader) (string, error) {
	var raw []byte
	var err error
	if f, ok := r.(*os.File); ok && term.IsTerminal(int(f.Fd())) { // #nosec G115 -- a file descriptor fits in int
		fmt.Fprint(os.Stderr, "Paste the API token: ")
		raw, err = term.ReadPassword(int(f.Fd())) // #nosec G115
		fmt.Fprintln(os.Stderr)
	} else {
		raw, err = io.ReadAll(io.LimitReader(bufio.NewReader(r), 4096))
	}
	if err != nil {
		return "", errors.Wrap(err, "read the token")
	}
	token := strings.TrimSpace(string(raw))
	if !strings.HasPrefix(token, store.TokenPrefix) {
		return "", errors.Errorf("a personal API token starts with %s", store.TokenPrefix)
	}
	return token, nil
}

func entryFor(token string, u client.MeUser, t client.Token, host string) credentials.Entry {
	return credentials.Entry{Token: token, TokenID: t.ID, Name: t.Name, Scopes: t.Scopes, ExpiresAt: t.ExpiresAt, SavedAt: time.Now().UTC(), Host: host,
		User: credentials.User{ID: u.ID, Name: u.Name, Email: u.Email, Role: u.Role}}
}

// saveLogin stores e for origin and prints it.
func saveLogin(ctx context.Context, out middlewares.Processor, origin string, e credentials.Entry, makeDefault bool) error {
	f, warning, err := credentials.Load()
	if err != nil {
		return err
	}
	if warning != "" {
		fmt.Fprintln(os.Stderr, "francois: "+warning)
	}
	f.Put(origin, e, makeDefault)
	if err = credentials.Save(f); err != nil {
		return err
	}
	if p, err := credentials.Path(); err == nil {
		fmt.Fprintln(os.Stderr, "Saved to "+p)
	}
	return out.AddRow(ctx, loginRow(origin, e, f.Default == origin))
}

func loginRow(origin string, e credentials.Entry, isDefault bool) types.Row {
	expires := "never"
	if e.ExpiresAt != nil {
		expires = e.ExpiresAt.Format(time.DateOnly)
	}
	return types.NewRow(types.MRP("server", origin), types.MRP("default", isDefault), types.MRP("user", e.User.Name), types.MRP("email", e.User.Email),
		types.MRP("role", e.User.Role), types.MRP("scopes", strings.Join(e.Scopes, " ")), types.MRP("expires", expires), types.MRP("token_id", e.TokenID))
}

type authStatus struct{ *glazed.CommandDescription }

var _ glazed.GlazeCommand = (*authStatus)(nil)

func (c *authStatus) RunIntoGlazeProcessor(ctx context.Context, v *values.Values, out middlewares.Processor) error {
	a := in{v}
	f, warning, err := credentials.Load()
	if err != nil {
		return err
	}
	if warning != "" {
		fmt.Fprintln(os.Stderr, "francois: "+warning)
	}
	only := ""
	if s := a.S("server"); s != "" {
		if only, err = credentials.NormalizeOrigin(s); err != nil {
			return err
		}
	}
	// What a command run now with the same --server would use.
	eff, effErr := credentials.Resolve(credentials.Inputs{Server: a.S("server")})
	origins := make([]string, 0, len(f.Servers))
	for o := range f.Servers {
		if only == "" || o == only {
			origins = append(origins, o)
		}
	}
	sort.Strings(origins)
	rows := 0
	for _, o := range origins {
		e := f.Servers[o]
		row := loginRow(o, e, f.Default == o)
		row.Set("used", usedBy(eff, effErr, o))
		if a.B("check") {
			row.Set("check", check(ctx, o, e.Token, e.Host))
		}
		if err := out.AddRow(ctx, row); err != nil {
			return err
		}
		rows++
	}
	if effErr == nil && eff.TokenSource != credentials.FromFile && (only == "" || eff.Server == only) {
		if _, stored := f.Servers[eff.Server]; !stored {
			row := types.NewRow(types.MRP("server", eff.Server), types.MRP("used", "yes, from "+describe(eff.TokenSource)))
			if a.B("check") {
				row.Set("check", check(ctx, eff.Server, eff.Token, eff.Host))
			}
			if err := out.AddRow(ctx, row); err != nil {
				return err
			}
			rows++
		}
	}
	if rows == 0 {
		fmt.Fprintln(os.Stderr, "Not logged in. Run francois catalog auth login --server URL.")
	}
	return nil
}

// usedBy says whether commands use the stored token for origin.
func usedBy(eff credentials.Resolved, err error, origin string) string {
	if err != nil || eff.Server != origin {
		return ""
	}
	if eff.TokenSource == credentials.FromFile {
		return "yes"
	}
	return "no, overridden by " + describe(eff.TokenSource)
}

func describe(s credentials.Source) string {
	switch s {
	case credentials.FromEnv:
		return "FRANCOIS_TOKEN"
	case credentials.FromTokenFile:
		return "--token-file"
	case credentials.FromFlag, credentials.FromFile:
	}
	return string(s)
}

// check asks the server about a token: "valid" or the error code.
func check(ctx context.Context, origin, token, host string) string {
	if _, err := client.New(origin, token, hostOption(host)...).Me(ctx); err != nil {
		var apiErr *client.Error
		if errors.As(err, &apiErr) {
			return apiErr.Code
		}
		return "unreachable"
	}
	return "valid"
}

type authToken struct{ *glazed.CommandDescription }

var _ glazed.BareCommand = (*authToken)(nil)

func (c *authToken) Run(_ context.Context, v *values.Values) error {
	a := in{v}
	res, err := credentials.Resolve(credentials.Inputs{Server: a.S("server")})
	if err != nil {
		return err
	}
	if term.IsTerminal(int(os.Stdout.Fd())) && !a.B("yes") { // #nosec G115 -- a file descriptor fits in int
		return errors.New("refusing to print a token to a terminal; redirect the output or pass --yes")
	}
	_, err = fmt.Fprintln(os.Stdout, res.Token)
	return err
}

type authLogout struct{ *glazed.CommandDescription }

var _ glazed.GlazeCommand = (*authLogout)(nil)

func (c *authLogout) RunIntoGlazeProcessor(ctx context.Context, v *values.Values, out middlewares.Processor) error {
	a := in{v}
	f, _, err := credentials.Load()
	if err != nil {
		return err
	}
	var targets []string
	if a.B("all") {
		for o := range f.Servers {
			targets = append(targets, o)
		}
		sort.Strings(targets)
	} else {
		srv, err := credentials.ResolveServer(a.S("server"))
		if err != nil {
			return err
		}
		if _, ok := f.Servers[srv.Server]; !ok {
			return errors.Errorf("not logged in to %s", srv.Server)
		}
		targets = []string{srv.Server}
	}
	for _, o := range targets {
		f.Remove(o)
		if err := out.AddRow(ctx, types.NewRow(types.MRP("server", o), types.MRP("logged_out", true))); err != nil {
			return err
		}
	}
	return credentials.Save(f)
}
