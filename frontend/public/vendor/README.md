# Local web assets

Outfit and Inter retain their existing font families and variable weights. Only
the Latin webfont subsets are bundled; other scripts use the system fallback.
The parent portal uses the pinned Chart.js 4.5.1 UMD distribution.

`manifest.json` records the exact npm package versions, upstream tarball SHA-512
integrity values, and extracted file SHA-384 hashes. Tarball integrity was
verified before extracting these files. Licenses are included alongside each
asset. The build verifies the local hashes and the portal's script integrity.

To update an asset, retrieve a specific version from the official npm registry,
verify the tarball against its registry integrity, then update the asset, license,
manifest, CSS or script references together. Do not replace these URLs with
unpinned CDN links. Run the frontend build and the public page browser checks.

Cloudflare Turnstile remains an external, dynamically updated anti-bot script;
its official endpoint is explicitly allowed by CSP. Its contents must not be
vendored or assigned a fixed integrity hash.
