import {readFile, mkdir, writeFile} from 'node:fs/promises';
import {createHash} from 'node:crypto';
import {fileURLToPath} from 'node:url';

const catalog = JSON.parse(await readFile(new URL('../docs/rules/catalog.json', import.meta.url), 'utf8'));
const directory = new URL('../local-rulebooks/', import.meta.url);
await mkdir(directory, {recursive: true});
const manifest = [];
let failures = 0;
const persist = () => writeFile(new URL('manifest.json', directory), JSON.stringify(manifest, null, 2));
for (const item of catalog) {
  if (!/^[a-z0-9.-]+\.pdf$/.test(item.filename)) throw new Error('Unsafe filename');
  const valid = bytes => bytes.length <= 30 * 1024 * 1024 && bytes.subarray(0, 1024).includes(Buffer.from('%PDF-'));
  let bytes, source = item.url, failure;
  try { const cached = await readFile(new URL(item.filename, directory)); if (valid(cached)) bytes = cached; } catch {}
  const urls = [item.url];
  if (new URL(item.url).hostname === 'hobbyworld.ru') urls.push(item.url.replace('://hobbyworld.ru/', '://hobbygames.ru/'));
  for (const url of bytes ? [] : urls) {
    try {
      const response = await fetch(url, {signal: AbortSignal.timeout(30000)});
      if (!response.ok) throw new Error(`HTTP ${response.status}`);
      const chunks = []; let size = 0;
      for await (const chunk of response.body) {
        size += chunk.length;
        if (size > 30 * 1024 * 1024) throw new Error('PDF exceeds 30 MB');
        chunks.push(chunk);
      }
      const downloaded = Buffer.concat(chunks);
      if (!valid(downloaded)) throw new Error('response is not a supported PDF');
      bytes = downloaded; source = url;
      await writeFile(new URL(item.filename, directory), bytes);
      break;
    } catch (error) { failure = error.message; console.warn(`${item.filename}: ${url}: ${failure}`); }
  }
  if (!bytes) {
    failures++;
    manifest.push({...item, error: failure || 'download failed'});
    await persist();
    continue;
  }
  const hash = createHash('sha256').update(bytes).digest('hex');
  manifest.push({...item, source, bytes: bytes.length, sha256: hash, verifiedAt: new Date().toISOString()});
  await persist();
  console.log(`${item.filename}: ${bytes.length} bytes, SHA256 ${hash}`);
}
console.log(`Collection: ${fileURLToPath(directory)}`);
if (failures) { console.error(`Failed to download ${failures} PDF(s); rerun to retry missing files.`); process.exitCode = 1; }
