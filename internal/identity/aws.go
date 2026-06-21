package identity

import (
	"context"
	"crypto/tls"
	"fmt"

	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/sts"
	"github.com/dscof/qm-agent/internal/config"
)

// Quartermaster subject_token_type URIs (see quartermaster/src/domain/identity/dispatcher.rs).
const (
	TokenTypeSPIREJWT        = "urn:ietf:params:oauth:token-type:jwt"
	TokenTypeAWSPresignedSTS = "urn:quartermaster:token-type:aws-presigned-sts"
	TokenTypeGCPIdentity     = "urn:quartermaster:token-type:gcp-identity"
)

type awsSource struct {
	cfg *config.Config
}

func newAWS(cfg *config.Config) (*awsSource, error) {
	return &awsSource{cfg: cfg}, nil
}

func (s *awsSource) Credential(ctx context.Context) (*Credential, error) {
	region := s.cfg.Identity.AWS.Region
	tokenType := s.cfg.Identity.AWS.SubjectTokenType

	awsCfg, err := awsconfig.LoadDefaultConfig(ctx, awsconfig.WithRegion(region))
	if err != nil {
		return nil, fmt.Errorf("load aws config: %w", err)
	}

	stsClient := sts.NewFromConfig(awsCfg)

	presigned, err := sts.NewPresignClient(stsClient).
		PresignGetCallerIdentity(ctx, &sts.GetCallerIdentityInput{})
	if err != nil {
		return nil, fmt.Errorf("presign get-caller-identity: %w", err)
	}

	return &Credential{
		SubjectToken:     presigned.URL,
		SubjectTokenType: tokenType,
	}, nil
}

func (s *awsSource) TLSConfig(ctx context.Context) (*tls.Config, error) {
	return tlsConfigFromFiles(s.cfg.Quartermaster.MTLS)
}

func (s *awsSource) Close() error {
	return nil
}
