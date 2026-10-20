# Hosting the read-only demo on Cloudflare Pages

This guide uses the repository's Git integration. The demo is a static export of
the production frontend with a browser-side mock API. It does not require a
backend, database, AI key, SMTP secret, or any other credential.

## Cloudflare Pages settings

Connect the repository in **Workers & Pages → Create → Pages → Connect to Git**,
then use:

| Setting | Value |
|---------|-------|
| Repository | `p1temax/Spearhook` |
| Production branch | `main` |
| Framework preset | `None` |
| Root directory | `frontend` |
| Build command | `npm ci && npm run build:demo:root` |
| Build output directory | `demo-dist` |

Add one environment variable:

```text
NODE_VERSION=22
```

Do **not** add `NEXT_PUBLIC_BASE_PATH` (it must be empty for `*.pages.dev` or a
root custom domain). Do not add any API keys, database URLs, SMTP credentials,
JWT secrets, or `config.yaml` values to the Pages environment.

The `build:demo:root` script sets the required public flags internally:

```text
NEXT_OUTPUT_EXPORT=1
NEXT_PUBLIC_DEMO_MODE=1
```

`404.html` is copied from the exported index so deep SPA links render through the
frontend router.

## Security checklist

- The demo uses mock data only; `frontend/src/demo/mock.ts` contains no real
  credentials, tokens, customer records, or production endpoints.
- Login is prefilled as `demo` / `demo`; any credentials are accepted by the
  browser-side mock adapter. This is intentional and is not an authentication
  boundary.
- `frontend/.env*` is gitignored. Do not commit environment files.
- `config.yaml`, `data/`, private keys, databases, and logs are gitignored.
- Cloudflare Pages build environment should contain only public build settings
  such as `NODE_VERSION`.
- If the demo must be private, add a Cloudflare Access policy to the Pages custom
  domain instead of putting secrets in the build.

## Custom domain

Add the domain under **Pages → Custom domains**. A subdomain such as
`demo.example.com` is sufficient; no base path is needed.

## Updating

Push to `main`; Cloudflare Pages rebuilds automatically. For local verification:

```bash
cd frontend
npm ci
npm run build:demo:root
```

The output is `frontend/demo-dist` and is ignored by Git.
