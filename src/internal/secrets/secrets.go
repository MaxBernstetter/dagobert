// Package secrets loads Bybit credentials from SSM and/or a .secrets env file.
package secrets

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ssm"

	"github.com/MaxBernstetter/dagobert/src/internal/awsenv"
)

// Env maps KEY=VALUE pairs for docker --env-file / process env.
type Env map[string]string

// LoadBybit returns BYBIT_* secrets. Order for auto:
//  1. Process env already set (non-empty)
//  2. SSM (when backend is ssm/auto and reachable)
//  3. SECRETS_FILE / .secrets file
func LoadBybit(ctx context.Context, secretsFile string) (Env, error) {
	out := Env{}
	mergeProcess(out)

	backend := awsenv.SecretsBackend()
	needSSM := backend == "ssm" || (backend == "auto" && (awsenv.Endpoint() != "" || os.Getenv("SSM_PREFIX") != ""))
	if needSSM {
		if err := mergeSSM(ctx, out); err != nil && backend == "ssm" {
			return nil, err
		}
	}

	if secretsFile != "" {
		_ = mergeFile(out, secretsFile)
	}

	return out, nil
}

// WriteEnvFile writes KEY=VALUE lines to a temp file for docker --env-file.
func WriteEnvFile(env Env) (string, error) {
	var lines []string
	for _, k := range []string{"BYBIT_API_KEY", "BYBIT_API_SECRET", "BYBIT_BASE_URL"} {
		if v := strings.TrimSpace(env[k]); v != "" {
			lines = append(lines, k+"="+v)
		}
	}
	if len(lines) == 0 {
		return "", fmt.Errorf("no BYBIT secrets resolved (SSM / .secrets / env)")
	}
	f, err := os.CreateTemp("", "execution-env-*")
	if err != nil {
		return "", err
	}
	if _, err := f.WriteString(strings.Join(lines, "\n") + "\n"); err != nil {
		f.Close()
		os.Remove(f.Name())
		return "", err
	}
	if err := f.Close(); err != nil {
		os.Remove(f.Name())
		return "", err
	}
	return f.Name(), nil
}

func mergeProcess(out Env) {
	for _, k := range []string{"BYBIT_API_KEY", "BYBIT_API_SECRET", "BYBIT_BASE_URL"} {
		if v := strings.TrimSpace(os.Getenv(k)); v != "" {
			out[k] = v
		}
	}
}

func mergeSSM(ctx context.Context, out Env) error {
	cfg, err := awsenv.Load(ctx)
	if err != nil {
		return err
	}
	client := ssm.NewFromConfig(cfg, func(o *ssm.Options) {
		if ep := awsenv.ResolveEndpoint(); ep != nil {
			o.BaseEndpoint = ep
		}
	})
	prefix := awsenv.SSMPrefix()
	names := []string{
		prefix + "/BYBIT_API_KEY",
		prefix + "/BYBIT_API_SECRET",
		prefix + "/BYBIT_BASE_URL",
		prefix + "/BINANCE_BASE_URL",
	}
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	res, err := client.GetParameters(ctx, &ssm.GetParametersInput{
		Names:          names,
		WithDecryption: aws.Bool(true),
	})
	if err != nil {
		return err
	}
	for _, p := range res.Parameters {
		if p.Name == nil || p.Value == nil {
			continue
		}
		name := *p.Name
		val := strings.TrimSpace(*p.Value)
		if val == "" || val == "replace-me" {
			continue
		}
		switch {
		case strings.HasSuffix(name, "/BYBIT_API_KEY"):
			if out["BYBIT_API_KEY"] == "" {
				out["BYBIT_API_KEY"] = val
			}
		case strings.HasSuffix(name, "/BYBIT_API_SECRET"):
			if out["BYBIT_API_SECRET"] == "" {
				out["BYBIT_API_SECRET"] = val
			}
		case strings.HasSuffix(name, "/BYBIT_BASE_URL"):
			if out["BYBIT_BASE_URL"] == "" {
				out["BYBIT_BASE_URL"] = val
			}
		}
	}
	return nil
}

func mergeFile(out Env, path string) error {
	raw, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	for _, line := range strings.Split(string(raw), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		line = strings.TrimPrefix(line, "export ")
		eq := strings.IndexByte(line, '=')
		if eq <= 0 {
			continue
		}
		k := strings.TrimSpace(line[:eq])
		v := strings.TrimSpace(line[eq+1:])
		if len(v) >= 2 {
			if (v[0] == '"' && v[len(v)-1] == '"') || (v[0] == '\'' && v[len(v)-1] == '\'') {
				v = v[1 : len(v)-1]
			}
		}
		if out[k] == "" && v != "" {
			out[k] = v
		}
	}
	return nil
}
