# US-5.7 — Config loading from file + env overrides

## Epic
E5: CLI

## Goal
Load Loom configuration from `~/.loom/config.toml` with field-by-field overrides from `LOOM_OWNER`, `LOOM_REPO`, `LOOM_TOKEN`, and `LOOM_DB_PATH` environment variables so every CLI subcommand shares a single, consistent config source.

## Acceptance Criteria

```
Given `~/.loom/config.toml` contains `owner = "acme"` and `repo = "myrepo"`
When `config.Load()` is called with no environment variables set
Then `cfg.Owner` is `"acme"` and `cfg.Repo` is `"myrepo"`
```

```
Given `LOOM_OWNER=override-owner` is set in the environment
When `config.Load()` is called
Then `cfg.Owner` is `"override-owner"` regardless of the file value
```

```
Given no config file exists and no environment variables are set
When `config.Load()` is called
Then it returns a zero-value `Config` with no error (missing file is not an error)
```

```
Given `LOOM_TOKEN` is set in the environment
When `config.Load()` is called
Then `cfg.Token` equals the env value
  And the token is never written to any log output
```

## Tasks

1. [ ] Write `config_test.go` with file-only, env-override, missing-file, and token-masking cases (write tests first)
2. [ ] Define `Config` struct in `internal/config/config.go` with fields: `Owner`, `Repo`, `Token`, `DBPath`
3. [ ] Implement `Load() (Config, error)` reading `~/.loom/config.toml` using `go-toml`
4. [ ] Apply env-variable overrides after file load using `os.Getenv`
5. [ ] Default `DBPath` to `.loom/state.db` if unset
6. [ ] Run `go test ./internal/config/... -race` and confirm green

## Dependencies
- US-1.2

## Size Estimate
S
