# Deploying a monobin site

monobin compiles to a single self-contained binary — no Node, no `node_modules`
on the server. Two common setups:

## A. SSR (`serve`) behind Caddy + systemd

```bash
# 1. cross-compile for the server
GOOS=linux GOARCH=amd64 go build -o monobin .

# 2. ship it
scp monobin server:/opt/mysite/monobin

# 3. run it under systemd
scp examples/deploy/monobin.service server:/etc/systemd/system/
ssh server 'systemctl daemon-reload && systemctl enable --now monobin'

# 4. put Caddy in front (see Caddyfile -> reverse_proxy 127.0.0.1:3000)
```

The binary serves everything from its embedded copy; the working directory and
Node are irrelevant in production.

## B. Static export on any file host

```bash
monobin build dist
```

Serve `dist/` with Caddy's `file_server` (the commented block in `Caddyfile`),
or upload to Netlify / Cloudflare Pages / R2 / S3. The generated `_redirects`
file is honored by Netlify and Cloudflare Pages; `404.html` is the fallback page;
`sitemap.xml` and `robots.txt` are emitted too (set `App.SiteURL` to your domain).
