# github-token-broker

`github-token-broker` is an AWS Lambda function that mints a short-lived
GitHub App installation token for `GilmanLab/secrets`.

The broker intentionally does not clone the repository, return secret files, or
decrypt SOPS. It only returns a temporary GitHub token with `contents:read` for
the `secrets` repository. Callers fetch encrypted files with `git` or the
GitHub Contents API, then decrypt locally with their own AWS KMS permissions.

## Configuration

Required environment variables:

- `AWS_REGION`

Optional environment variables:

- `GITHUB_TOKEN_BROKER_CLIENT_ID_PARAM`, default `/glab/bootstrap/github-app/client-id`
- `GITHUB_TOKEN_BROKER_INSTALLATION_ID_PARAM`, default `/glab/bootstrap/github-app/installation-id`
- `GITHUB_TOKEN_BROKER_PRIVATE_KEY_PARAM`, default `/glab/bootstrap/github-app/private-key-pem`
- `GITHUB_TOKEN_BROKER_GITHUB_API_BASE_URL`, default `https://api.github.com`
- `GITHUB_TOKEN_BROKER_LOG_LEVEL`, default `info`

## Local Checks

```sh
just check
```

## Invocation Contract

Invoke the Lambda with an empty or `null` payload. The broker does not accept
caller-selected repositories, permissions, paths, or refs.

## No-Git Bootstrap Fetch

After invoking the Lambda, a bootstrap caller can use the returned token with
the GitHub Contents API:

```sh
curl -fsSL \
  -H "Accept: application/vnd.github.raw" \
  -H "Authorization: Bearer ${GITHUB_TOKEN}" \
  "https://api.github.com/repos/GilmanLab/secrets/contents/path/to/file.sops.yaml?ref=master" \
  -o file.sops.yaml
```
