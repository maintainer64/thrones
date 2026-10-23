import assert from 'node:assert/strict';
import {execFileSync} from 'node:child_process';
import {setTimeout as pause} from 'node:timers/promises';

const base = process.env.BASE_URL || 'http://127.0.0.1:18081';
const restart = process.argv.includes('--restart');
if (restart && !['localhost', '127.0.0.1'].includes(new URL(base).hostname)) throw new Error('Restart smoke must target localhost');
async function api(path, body, cookie) {
  const response = await fetch(base + path, {method: body ? 'POST' : 'GET', headers: {...(body ? {'Content-Type': 'application/json'} : {}), ...(cookie ? {Cookie: cookie} : {})}, body: body ? JSON.stringify(body) : undefined});
  const value = await response.json();
  assert.ok(response.ok, `${path}: HTTP ${response.status}: ${JSON.stringify(value)}`);
  return {value, cookie: response.headers.get('set-cookie')?.split(';')[0] || cookie};
}
assert.equal((await api('/api/health')).value.server, 'go');
const html = await fetch(base + '/').then(r => r.text());
assert.match(html, /id="root"/);
const catalog = (await api('/api/catalog')).value;
assert.ok((await fetch(base + '/media/' + catalog.board.image)).ok);
let {value: room, cookie: host} = await api('/api/rooms', {name: 'Smoke host', title: 'Проверка контейнера'});
const path = '/api/rooms/' + room.id;
async function command(cookie, action) {
  room = (await api(path + '/commands', {...action, revision: room.revision}, cookie)).value;
  return room;
}
await command(host, {type: 'configure', house: 'greyjoy', bot: false, difficulty: 'normal'});
const {cookie: guest} = await api(path + '/join', {name: 'Smoke guest', house: 'greyjoy'});
room = (await api(path, undefined, host)).value;
await command(host, {type: 'start'});
assert.equal(room.phase, 'planning');
assert.deepEqual(room.orders, {});
const tokens = ['march', 'defend-1', 'support-1', 'power-1'];
for (const [house, cookie] of [['stark', host], ['greyjoy', guest]]) {
  const regions = Object.entries(catalog.units).filter(([, army]) => army.some(u => u.house === house)).map(([id]) => id);
  for (const [index, region] of regions.entries()) await command(cookie, {type: 'order', region, order: tokens[index]});
  const other = (await api(path, undefined, house === 'stark' ? guest : host)).value;
  for (const region of regions) assert.equal(other.orders[region], undefined, 'secret order leaked');
}
await command(host, {type: 'ready'});
assert.equal(room.phase, 'planning');
await command(guest, {type: 'ready'});
assert.equal(room.phase, 'orders_revealed');
assert.ok(Object.keys(room.orders).length > 6);
assert.equal((await fetch(base + path)).status, 403);
if (restart) {
  execFileSync('docker', ['compose', 'restart', 'app'], {cwd: new URL('../', import.meta.url), stdio: 'inherit'});
  let healthy = false;
  for (let i = 0; i < 40; i++) {
    try { if ((await fetch(base + '/api/health')).ok) {healthy = true; break;} } catch {}
    await pause(250);
  }
  assert.ok(healthy, 'container did not recover');
  const restored = (await api(path, undefined, host)).value;
  assert.equal(restored.revision, room.revision);
  assert.deepEqual(restored.orders, room.orders);
  assert.equal(restored.phase, 'orders_revealed');
  assert.equal(restored.yourHouse, 'stark');
}
console.log(`Smoke passed: two anonymous players, private orders, simultaneous reveal${restart ? ', container restart and SQLite restore' : ''}.`);
console.log(`Test room: ${base}/?room=${room.id} (created with test identities, not your browser).`);
