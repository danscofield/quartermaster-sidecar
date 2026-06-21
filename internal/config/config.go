package config

import (
	"fmt"
	"os"
	"time"

	"gopkg.in/yaml.v3"
)

// Config is the daemon configuration.
type Config struct {
	Quartermaster QuartermasterConfig `yaml:"quartermaster"`
	Identity      IdentityConfig      `yaml:"identity"`
	Exchange      ExchangeConfig      `yaml:"exchange"`
	Refresh       RefreshConfig       `yaml:"refresh"`
	CSR           CSRConfig           `yaml:"csr"`
	Server        ServerConfig        `yaml:"server"`
}

type QuartermasterConfig struct {
	URL  string     `yaml:"url"`
	MTLS MTLSConfig `yaml:"mtls"`
}

type MTLSConfig struct {
	CertFile string `yaml:"cert_file"`
	KeyFile  string `yaml:"key_file"`
	CAFile   string `yaml:"ca_file"`
}

type IdentityConfig struct {
	Type  string       `yaml:"type"`
	SPIRE *SPIREConfig `yaml:"spire,omitempty"`
	AWS   *AWSConfig   `yaml:"aws,omitempty"`
	GCP   *GCPConfig   `yaml:"gcp,omitempty"`
}

type SPIREConfig struct {
	Mode        string `yaml:"mode"`
	SocketPath  string `yaml:"socket_path"`
	JWTAudience string `yaml:"jwt_audience"`
}

type AWSConfig struct {
	Region           string `yaml:"region"`
	SubjectTokenType string `yaml:"subject_token_type"`
}

type GCPConfig struct {
	Audience         string `yaml:"audience"`
	MetadataHost     string `yaml:"metadata_host"`
	SubjectTokenType string `yaml:"subject_token_type"`
}

type ExchangeConfig struct {
	Billets          []string `yaml:"billets"`
	GrantType        string   `yaml:"grant_type"`
	Audience         string   `yaml:"audience"`
	SubjectTokenType string   `yaml:"subject_token_type"`
}

type RefreshConfig struct {
	Margin time.Duration `yaml:"margin"`
}

type CSRConfig struct {
	Enabled bool `yaml:"enabled"`
}

type ServerConfig struct {
	// Listen address for the credential HTTP API (e.g. 127.0.0.1:8765).
	Listen string `yaml:"listen"`
}

// Load reads and validates configuration from a YAML file.
func Load(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	cfg := &Config{}
	if err := yaml.Unmarshal(data, cfg); err != nil {
		return nil, err
	}
	cfg.applyDefaults()
	if err := cfg.validate(); err != nil {
		return nil, err
	}
	return cfg, nil
}

func (c *Config) applyDefaults() {
	if c.Refresh.Margin == 0 {
		c.Refresh.Margin = 5 * time.Minute
	}
	if c.Exchange.GrantType == "" {
		c.Exchange.GrantType = "urn:ietf:params:oauth:grant-type:token-exchange"
	}
	if c.Exchange.Audience == "" && c.Quartermaster.URL != "" {
		c.Exchange.Audience = c.Quartermaster.URL
	}
	if c.Server.Listen == "" {
		c.Server.Listen = "127.0.0.1:8765"
	}
	if c.Identity.SPIRE != nil {
		if c.Identity.SPIRE.Mode == "" {
			c.Identity.SPIRE.Mode = "mtls"
		}
		if c.Identity.SPIRE.SocketPath == "" {
			c.Identity.SPIRE.SocketPath = "unix:///tmp/spire-agent/public/api.sock"
		}
	}
	if c.Identity.AWS != nil && c.Identity.AWS.SubjectTokenType == "" {
		c.Identity.AWS.SubjectTokenType = "urn:quartermaster:token-type:aws-presigned-sts"
	}
	if c.Identity.GCP != nil {
		if c.Identity.GCP.MetadataHost == "" {
			c.Identity.GCP.MetadataHost = "metadata.google.internal"
		}
		if c.Identity.GCP.SubjectTokenType == "" {
			c.Identity.GCP.SubjectTokenType = "urn:quartermaster:token-type:gcp-identity"
		}
	}
}

func (c *Config) validate() error {
	if c.Quartermaster.URL == "" {
		return fmt.Errorf("quartermaster.url is required")
	}

	switch c.Identity.Type {
	case "spire":
		if c.Identity.SPIRE == nil {
			return fmt.Errorf("identity.spire is required when identity.type is spire")
		}
		if c.Identity.SPIRE.Mode == "jwt" && c.Identity.SPIRE.JWTAudience == "" {
			return fmt.Errorf("identity.spire.jwt_audience is required when mode is jwt")
		}
	case "aws":
		if c.Identity.AWS == nil {
			return fmt.Errorf("identity.aws is required when identity.type is aws")
		}
		if c.Identity.AWS.Region == "" {
			return fmt.Errorf("identity.aws.region is required")
		}
	case "gcp":
		if c.Identity.GCP == nil {
			return fmt.Errorf("identity.gcp is required when identity.type is gcp")
		}
		if c.Identity.GCP.Audience == "" {
			return fmt.Errorf("identity.gcp.audience is required")
		}
	default:
		return fmt.Errorf("identity.type must be spire, aws, or gcp")
	}

	if c.Server.Listen == "" {
		return fmt.Errorf("server.listen is required")
	}
	if c.Exchange.Audience == "" {
		return fmt.Errorf("exchange.audience is required for token exchange")
	}
	if c.Quartermaster.MTLS.CertFile != "" || c.Quartermaster.MTLS.KeyFile != "" {
		if c.Quartermaster.MTLS.CertFile == "" || c.Quartermaster.MTLS.KeyFile == "" {
			return fmt.Errorf("quartermaster.mtls.cert_file and key_file must both be set")
		}
	}
	return nil
}
