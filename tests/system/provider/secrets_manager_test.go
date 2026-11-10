package provider_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"connectrpc.com/connect"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/secretsmanager"

	"github.com/MaxBernstetter/dagobert/src/experimentapi"
	"github.com/MaxBernstetter/dagobert/src/experimentapi/providers"
	v1 "github.com/MaxBernstetter/dagobert/src/gen/tradingcopilot/v1"
	"github.com/MaxBernstetter/dagobert/src/gen/tradingcopilot/v1/tradingcopilotv1connect"
	"github.com/MaxBernstetter/dagobert/src/shared/pgtest"
	"github.com/MaxBernstetter/dagobert/tests/system/internal/floci"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

// TestProviderSecretsInSecretsManager runs Floci (Secrets Manager) and a scratch Postgres, serves ProviderService
// through the Experiment API mount, and checks: the secret lands in Secrets Manager under
// /dagobert/<env>/providers/<id> as a JSON object, the Connect view reports only the key name, and a config save
// is read back.
func TestProviderSecretsInSecretsManager(t *testing.T) {
	bucket := floci.BucketName("provider-test")
	endpoint, _ := floci.Start(t, bucket)
	prefix := "/dagobert/test-" + uuid.NewString()[:8]
	t.Setenv("AWS_ENDPOINT_URL", endpoint)
	t.Setenv("AWS_REGION", "us-east-1")
	t.Setenv("SECRETS_BACKEND", "secretsmanager")
	t.Setenv("SSM_PREFIX", prefix)
	t.Setenv("DATA_DIR", t.TempDir())

	dbURL := pgtest.ScratchDB(t)
	pool, err := pgxpool.New(t.Context(), dbURL)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pool.Close)

	// The API reads the secret through the env-selected backend, the same one the test writes with.
	cfg, err := config.LoadDefaultConfig(t.Context(),
		config.WithRegion("us-east-1"),
		config.WithCredentialsProvider(credentials.NewStaticCredentialsProvider("test", "test", "")))
	if err != nil {
		t.Fatal(err)
	}
	sm := secretsmanager.NewFromConfig(cfg, func(o *secretsmanager.Options) { o.BaseEndpoint = aws.String(endpoint) })
	const secret = "s3cr3t-value-456"
	t.Cleanup(func() {
		_, _ = sm.DeleteSecret(context.Background(), &secretsmanager.DeleteSecretInput{
			SecretId: aws.String(prefix + "/providers/binance"), ForceDeleteWithoutRecovery: aws.Bool(true),
		})
	})
	store := providers.NewSecretsManager(sm, prefix)
	if err := store.Put(t.Context(), "binance", map[string]string{"apiKey": secret}); err != nil {
		t.Fatalf("put secret: %v", err)
	}

	raw, err := sm.GetSecretValue(t.Context(), &secretsmanager.GetSecretValueInput{SecretId: aws.String(prefix + "/providers/binance")})
	if err != nil {
		t.Fatalf("secret not in Secrets Manager under the provider name: %v", err)
	}
	var stored map[string]string
	if err := json.Unmarshal([]byte(aws.ToString(raw.SecretString)), &stored); err != nil || stored["apiKey"] != secret {
		t.Fatalf("secret JSON %q: %v", aws.ToString(raw.SecretString), err)
	}

	mux := http.NewServeMux()
	experimentapi.Mount(mux, experimentapi.NewServer(pool, experimentapi.Hooks{}))
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	client := tradingcopilotv1connect.NewProviderServiceClient(srv.Client(), srv.URL)

	view, err := client.GetProvider(t.Context(), connect.NewRequest(&v1.GetProviderRequest{Id: "binance"}))
	if err != nil {
		t.Fatalf("get provider: %v", err)
	}
	keys := view.Msg.GetProvider().GetSecretKeysConfigured()
	if len(keys) != 1 || keys[0] != "apiKey" {
		t.Fatalf("secret keys %v want [apiKey]", keys)
	}
	if body, _ := json.Marshal(view.Msg); strings.Contains(string(body), secret) {
		t.Fatal("the secret value leaked into the provider view")
	}

	saved, err := client.SaveProviderSettings(t.Context(), connect.NewRequest(&v1.SaveProviderSettingsRequest{
		Id:     "binance",
		Config: map[string]string{"baseUrl": "https://example.invalid"},
	}))
	if err != nil {
		t.Fatalf("save config: %v", err)
	}
	if got := saved.Msg.GetProvider().GetConfig()["baseUrl"]; got != "https://example.invalid" {
		t.Fatalf("saved baseUrl %q", got)
	}
	if _, err := client.SaveProviderSettings(t.Context(), connect.NewRequest(&v1.SaveProviderSettingsRequest{
		Id: "binance", Secrets: map[string]string{"apiKey": "not-declared"},
	})); connect.CodeOf(err) != connect.CodeInvalidArgument {
		t.Fatalf("undeclared secret key: %v", err)
	}
}
