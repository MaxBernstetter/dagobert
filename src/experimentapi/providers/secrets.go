package providers

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"maps"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/secretsmanager"
	"github.com/aws/aws-sdk-go-v2/service/secretsmanager/types"

	"github.com/MaxBernstetter/dagobert/src/internal/awsenv"
)

// SecretStore holds the write-only secret values of each provider. Values are never returned to a client.
type SecretStore interface {
	// Get returns the stored values of a provider; a provider without secrets is an empty map.
	Get(ctx context.Context, providerID string) (map[string]string, error)
	// Put merges values into the stored ones of a provider.
	Put(ctx context.Context, providerID string, values map[string]string) error
}

// DataDir is the root of the file backends: LIVE_DIR, else DATA_DIR, else "data".
func DataDir() string {
	if d := strings.TrimSpace(os.Getenv("LIVE_DIR")); d != "" {
		return d
	}
	if d := strings.TrimSpace(os.Getenv("DATA_DIR")); d != "" {
		return d
	}
	return "data"
}

// SecretsFromEnv picks the secret backend. SECRETS_BACKEND is "file" (alias "fs"), "secretsmanager" or "auto".
// Auto is secretsmanager when AWS_ENDPOINT_URL or SSM_PREFIX is set (Floci or AWS), else the file under DATA_DIR.
func SecretsFromEnv(ctx context.Context) (SecretStore, error) {
	backend := strings.ToLower(strings.TrimSpace(os.Getenv("SECRETS_BACKEND")))
	if backend == "" || backend == "auto" {
		backend = "file"
		if awsenv.Endpoint() != "" || strings.TrimSpace(os.Getenv("SSM_PREFIX")) != "" {
			backend = "secretsmanager"
		}
	}
	switch backend {
	case "file", "fs":
		return FileSecrets{Dir: DataDir()}, nil
	case "secretsmanager":
		cfg, err := awsenv.Load(ctx)
		if err != nil {
			return nil, err
		}
		client := secretsmanager.NewFromConfig(cfg, func(o *secretsmanager.Options) {
			if ep := awsenv.ResolveEndpoint(); ep != nil {
				o.BaseEndpoint = ep
			}
		})
		return NewSecretsManager(client, awsenv.SSMPrefix()), nil
	default:
		return nil, fmt.Errorf("unknown SECRETS_BACKEND %q", backend)
	}
}

// unavailableSecrets answers every call with the reason the backend could not be built.
type unavailableSecrets struct{ err error }

// Unavailable is the SecretStore of a process whose backend failed to start: every call returns err.
func Unavailable(err error) SecretStore { return unavailableSecrets{err: err} }

func (u unavailableSecrets) Get(context.Context, string) (map[string]string, error) {
	return nil, u.err
}

func (u unavailableSecrets) Put(context.Context, string, map[string]string) error { return u.err }

// MergeSecrets returns stored with values written over it. Values are trimmed.
func MergeSecrets(stored, values map[string]string) map[string]string {
	out := map[string]string{}
	maps.Copy(out, stored)
	for k, v := range values {
		out[k] = strings.TrimSpace(v)
	}
	return out
}

// ConfiguredKeys returns the sorted names of the secrets with a non-blank value. Never the values.
func ConfiguredKeys(values map[string]string) []string {
	keys := []string{}
	for k, v := range values {
		if strings.TrimSpace(v) != "" {
			keys = append(keys, k)
		}
	}
	slices.Sort(keys)
	return keys
}

// EncodeSecret is the Secrets Manager secret string: a JSON object of key to value.
func EncodeSecret(values map[string]string) (string, error) {
	body, err := json.Marshal(values)
	return string(body), err
}

// DecodeSecret reads a Secrets Manager secret string. A blank string is an empty set.
func DecodeSecret(raw string) (map[string]string, error) {
	out := map[string]string{}
	if strings.TrimSpace(raw) == "" {
		return out, nil
	}
	if err := json.Unmarshal([]byte(raw), &out); err != nil {
		return nil, fmt.Errorf("secret is not a JSON object of strings: %w", err)
	}
	return out, nil
}

// FileSecrets keeps the values in DATA_DIR/providers/secrets.json (mode 0600, written atomically).
type FileSecrets struct {
	Dir string
}

type secretsFile struct {
	Providers map[string]map[string]string `json:"providers"`
}

var fileMu sync.Mutex

func (f FileSecrets) path() string { return filepath.Join(f.Dir, "providers", "secrets.json") }

func (f FileSecrets) read() (secretsFile, error) {
	var doc secretsFile
	raw, err := os.ReadFile(f.path())
	if errors.Is(err, os.ErrNotExist) {
		return doc, nil
	}
	if err != nil {
		return doc, err
	}
	if err := json.Unmarshal(raw, &doc); err != nil {
		return doc, fmt.Errorf("providers/secrets.json is not valid JSON")
	}
	return doc, nil
}

// Get returns the values of a provider from the file.
func (f FileSecrets) Get(_ context.Context, providerID string) (map[string]string, error) {
	fileMu.Lock()
	defer fileMu.Unlock()
	doc, err := f.read()
	if err != nil {
		return nil, err
	}
	out := map[string]string{}
	maps.Copy(out, doc.Providers[providerID])
	return out, nil
}

// Put merges values into the file and replaces it atomically (temp file, then rename).
func (f FileSecrets) Put(_ context.Context, providerID string, values map[string]string) error {
	fileMu.Lock()
	defer fileMu.Unlock()
	doc, err := f.read()
	if err != nil {
		return err
	}
	if doc.Providers == nil {
		doc.Providers = map[string]map[string]string{}
	}
	doc.Providers[providerID] = MergeSecrets(doc.Providers[providerID], values)
	body, err := json.MarshalIndent(doc, "", "  ")
	if err != nil {
		return err
	}
	return writeFileAtomic(f.path(), append(body, '\n'))
}

func writeFileAtomic(path string, body []byte) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(dir, ".secrets-*.json")
	if err != nil {
		return err
	}
	defer os.Remove(tmp.Name())
	if err := tmp.Chmod(0o600); err != nil {
		tmp.Close()
		return err
	}
	if _, err := tmp.Write(body); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Sync(); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	return os.Rename(tmp.Name(), path)
}

// SecretsManager keeps one secret per provider: <prefix>/providers/<id>, a JSON object of key to value.
type SecretsManager struct {
	client *secretsmanager.Client
	prefix string
}

// NewSecretsManager builds the store. prefix is e.g. /dagobert/prod (see awsenv.SSMPrefix).
func NewSecretsManager(client *secretsmanager.Client, prefix string) *SecretsManager {
	return &SecretsManager{client: client, prefix: strings.TrimRight(prefix, "/")}
}

// SecretName is the Secrets Manager name of a provider's secret.
func (m *SecretsManager) SecretName(providerID string) string {
	return m.prefix + "/providers/" + providerID
}

// Get reads the secret of a provider; a missing secret is an empty set.
func (m *SecretsManager) Get(ctx context.Context, providerID string) (map[string]string, error) {
	out, err := m.client.GetSecretValue(ctx, &secretsmanager.GetSecretValueInput{
		SecretId: aws.String(m.SecretName(providerID)),
	})
	var notFound *types.ResourceNotFoundException
	if errors.As(err, &notFound) {
		return map[string]string{}, nil
	}
	if err != nil {
		return nil, err
	}
	return DecodeSecret(aws.ToString(out.SecretString))
}

// Put merges values into the secret, creating it on the first write.
func (m *SecretsManager) Put(ctx context.Context, providerID string, values map[string]string) error {
	current, err := m.Get(ctx, providerID)
	if err != nil {
		return err
	}
	body, err := EncodeSecret(MergeSecrets(current, values))
	if err != nil {
		return err
	}
	_, err = m.client.PutSecretValue(ctx, &secretsmanager.PutSecretValueInput{
		SecretId:     aws.String(m.SecretName(providerID)),
		SecretString: aws.String(body),
	})
	var notFound *types.ResourceNotFoundException
	if errors.As(err, &notFound) {
		_, err = m.client.CreateSecret(ctx, &secretsmanager.CreateSecretInput{
			Name:         aws.String(m.SecretName(providerID)),
			SecretString: aws.String(body),
		})
	}
	return err
}
