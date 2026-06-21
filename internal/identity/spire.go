package identity

import (
	"context"
	"crypto/tls"
	"fmt"
	"sync"

	"github.com/dscof/qm-agent/internal/config"
	"github.com/spiffe/go-spiffe/v2/spiffetls/tlsconfig"
	"github.com/spiffe/go-spiffe/v2/svid/jwtsvid"
	"github.com/spiffe/go-spiffe/v2/workloadapi"
)

type spireSource struct {
	mode        string
	jwtAudience string
	x509        *workloadapi.X509Source
	jwt         *workloadapi.JWTSource
	tlsOnce     sync.Once
	tlsCfg      *tls.Config
	tlsErr      error
}

func newSPIRE(ctx context.Context, cfg *config.Config) (*spireSource, error) {
	sc := cfg.Identity.SPIRE
	clientOpts := workloadapi.WithClientOptions(workloadapi.WithAddr(sc.SocketPath))

	s := &spireSource{
		mode:        sc.Mode,
		jwtAudience: sc.JWTAudience,
	}

	switch sc.Mode {
	case "mtls":
		x509, err := workloadapi.NewX509Source(ctx, clientOpts)
		if err != nil {
			return nil, fmt.Errorf("spire x509 source: %w", err)
		}
		s.x509 = x509
	case "jwt":
		jwt, err := workloadapi.NewJWTSource(ctx, clientOpts)
		if err != nil {
			return nil, fmt.Errorf("spire jwt source: %w", err)
		}
		s.jwt = jwt
	default:
		return nil, fmt.Errorf("identity.spire.mode must be mtls or jwt")
	}
	return s, nil
}

func (s *spireSource) Credential(ctx context.Context) (*Credential, error) {
	if s.mode != "jwt" {
		return nil, nil
	}
	svid, err := s.jwt.FetchJWTSVID(ctx, jwtsvid.Params{Audience: s.jwtAudience})
	if err != nil {
		return nil, fmt.Errorf("fetch jwt svid: %w", err)
	}
	return &Credential{
		SubjectToken:     svid.Marshal(),
		SubjectTokenType: TokenTypeSPIREJWT,
	}, nil
}

func (s *spireSource) TLSConfig(ctx context.Context) (*tls.Config, error) {
	if s.mode == "mtls" {
		s.tlsOnce.Do(func() {
			s.tlsCfg = tlsconfig.MTLSClientConfig(s.x509, s.x509, tlsconfig.AuthorizeAny())
		})
		return s.tlsCfg, s.tlsErr
	}
	return nil, nil
}

func (s *spireSource) Close() error {
	if s.x509 != nil {
		return s.x509.Close()
	}
	if s.jwt != nil {
		return s.jwt.Close()
	}
	return nil
}
