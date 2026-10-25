import {mkdir,cp,rm} from 'node:fs/promises';
const destination=new URL('../internal/assets/dist/',import.meta.url);
await mkdir(destination,{recursive:true});
// Remove only generated hashed bundles; never accumulate stale JavaScript in the Go binary.
await rm(new URL('assets/',destination),{recursive:true,force:true});
// Replace only this generated subdirectory, so obsolete P/M variants cannot
// linger in the embedded binary. Editable originals live in web/public/.
await rm(new URL('media/orders/',destination),{recursive:true,force:true});
// Legacy public PNG cards no longer have a renderer or a manifest entry.
await rm(new URL('media/cards/',destination),{recursive:true,force:true});
await cp(new URL('../web/dist/',import.meta.url),destination,{recursive:true});
console.log('Embedded frontend assets prepared');
