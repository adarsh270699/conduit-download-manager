// Bundles NumberFlow into the one browser file the daemon embeds.
// Dev-only: the daemon never runs this. Run `npm install` and `npm run vendor` here.
// The output and its LICENSE are committed under daemon/internal/server/web/vendor/.
import { build } from 'esbuild';
import { mkdir, readFile, writeFile } from 'node:fs/promises';
import path from 'node:path';
import { fileURLToPath } from 'node:url';

const here = path.dirname(fileURLToPath(import.meta.url));
const outDir = path.join(here, '..', '..', 'daemon', 'internal', 'server', 'web', 'vendor', 'number-flow');
const outFile = path.join(outDir, 'number-flow.js');
const pkgDir = path.join(here, 'node_modules', 'number-flow');

const pkg = JSON.parse(await readFile(path.join(pkgDir, 'package.json'), 'utf8'));

await mkdir(outDir, { recursive: true });

// The entry is the group build. It imports the full build (dist/index.mjs), which has update(value)
// and an Intl format option, and it defines <number-flow-group>, which batches the layout reads and
// writes of every NumberFlow inside it. The lite build only accepts pre-formatted parts.
await build({
  entryPoints: [path.join(pkgDir, 'dist', 'group.mjs')],
  bundle: true,
  format: 'esm',
  platform: 'browser',
  target: 'es2020',
  minify: true,
  legalComments: 'none',
  outfile: outFile,
  banner: {
    js: `/* NumberFlow ${pkg.version} (MIT). Vendored by tools/vendor-numberflow. Do not edit. */`,
  },
});

const license = `NumberFlow ${pkg.version}
Source: https://github.com/barvian/number-flow (npm: number-flow@${pkg.version})

MIT License

Copyright (c) ${pkg.author.name}

Permission is hereby granted, free of charge, to any person obtaining a copy
of this software and associated documentation files (the "Software"), to deal
in the Software without restriction, including without limitation the rights
to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
copies of the Software, and to permit persons to whom the Software is
furnished to do so, subject to the following conditions:

The above copyright notice and this permission notice shall be included in all
copies or substantial portions of the Software.

THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE
SOFTWARE.
`;
await writeFile(path.join(outDir, 'LICENSE'), license);

console.log(`Wrote ${path.relative(process.cwd(), outFile)} (NumberFlow ${pkg.version})`);
