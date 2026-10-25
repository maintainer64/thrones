import {readFile,mkdir,writeFile,cp,access} from 'node:fs/promises';
import {resolve,sep} from 'node:path';
import {fileURLToPath,pathToFileURL} from 'node:url';
import {parseArgs} from 'node:util';
const {values}=parseArgs({options:{source:{type:'string'},output:{type:'string'},help:{type:'boolean',short:'h'}}});
if(values.help){
 console.log('Usage: node scripts/export-game-data.mjs [--source <agot-bg-game-server>] [--output <directory>]\nDefault source: ../westeros-web-references/agot-bg-game-server (or WESTEROS_REFERENCE_DIR).\nDefault output: current project. Existing generated files are overwritten.');
 process.exit(0);
}
const referencePath=resolve(values.source ?? process.env.WESTEROS_REFERENCE_DIR ?? fileURLToPath(new URL('../../westeros-web-references/agot-bg-game-server/',import.meta.url)));
const sourceRoot=pathToFileURL(referencePath+sep);
const root=values.output?pathToFileURL(resolve(values.output)+sep):new URL('../',import.meta.url);
try{
 for(const file of ['data/baseGameData.json','data/westeros.json','data/westeros-with-essos.json','public']) await access(new URL(file,sourceRoot));
}catch(error){
 throw new Error(`Reference source is missing or incomplete: ${referencePath}. Use --source <agot-bg-game-server>. References are required only for re-export, not for building/running the game.`,{cause:error});
}
const base=JSON.parse(await readFile(new URL('data/baseGameData.json',sourceRoot),'utf8'));
const boards={};
for(const id of ['westeros','westeros-with-essos']){
  const map=JSON.parse(await readFile(new URL(`data/${id}.json`,sourceRoot),'utf8'));
  const layer=name=>map.layers.find(l=>l.name===name)?.objects ?? [];
  const props=o=>Object.fromEntries((o.properties || []).map(p=>[p.name,p.value]));
  const edges=layer('Borders').map(props);
  const slots=Object.fromEntries(layer('Order Slots').map(o=>[props(o).region,{x:o.x+o.width/2,y:o.y+o.height/2}]));
  const bg=layer('Background')[0];
  boards[id]={id,width:bg.width,height:bg.height,image:`${id}.jpg`,regions:layer('Regions').map(o=>{
    const p=props(o),s=slots[p.id] || {x:o.x,y:o.y};
    return {...p,id:p.id,name:o.name,x:s.x,y:s.y,neighbors:edges.flatMap(e=>e.from===p.id?[e.to]:e.to===p.id?[e.from]:[])};
  })};
  const regionIDs=new Set(boards[id].regions.map(r=>r.id));
  for(const region of boards[id].regions) region.neighbors=[...new Set(region.neighbors.filter(n=>n && n!==region.id && regionIDs.has(n)))];
}
const houses=new Set(base.setups['base-game'].playerSetups.find(s=>s.playerCount===6).houses);
const regionIDs=new Set(boards.westeros.regions.map(r=>r.id));
const units=Object.fromEntries(Object.entries(base.units).filter(([id])=>regionIDs.has(id)).map(([id,army])=>[id,army.filter(u=>houses.has(u.house) && u.quantity>0)]).filter(([,army])=>army.length));
const data={source:'Longwelwind/swords-and-ravens (GPL-3.0; artwork separately owned)',houses:base.houses,units,setups:base.setups,tracks:base.tracks,starredOrderRestrictions:base.starredOrderRestrictions,supplyRestrictions:base.supplyRestrictions,boards};
await mkdir(new URL('internal/game/',root),{recursive:true});
await writeFile(new URL('internal/game/catalog.json',root),JSON.stringify(data));
await mkdir(new URL('web/public/media/',root),{recursive:true});
for(const id of ['westeros','westeros-with-essos']) await cp(new URL(`public/images/${id}.jpg`,sourceRoot),new URL(`web/public/media/${id}.jpg`,root));
console.log('Exported map graph, initial armies, houses and setup data');

const registry={units:{},orders:{}};
const publicRoot=new URL('public/',sourceRoot);
const media=new URL('web/public/media/',root);
for(const directory of ['units','orders','crests','sounds']) await mkdir(new URL(directory+'/',media),{recursive:true});
const prefixes={stark:'Star',greyjoy:'Grey',lannister:'Lan',baratheon:'Barath',tyrell:'Tyr',martell:'Mart'};
for(const [house,prefix] of Object.entries(prefixes)) {
 registry.units[house]={};
 await cp(new URL(`images/icons/houses/${house}.svg`,publicRoot),new URL(`crests/${house}.svg`,media));
 for(const [type,suffix] of Object.entries({footman:'Foot',knight:'Knight','siege-engine':'Seige',ship:'Ship'})) {
  const filename=`${prefix}${suffix}.png`;await cp(new URL('images/units/'+filename,publicRoot),new URL('units/'+filename,media));registry.units[house][type]='/media/units/'+filename;
 }
}
const orderBases=JSON.parse(await readFile(new URL('../web/src/artwork/order-bases.json',import.meta.url),'utf8'));
const orderIDs=['march','march-minus','march-star','defend-1','defend-2','defend-star','support-1','support-2','support-star','raid-1','raid-2','raid-star','power-1','power-2','power-star'];
for(const id of orderIDs){
 const name=orderBases[id.split('-')[0]];
 if(!name)throw new Error(`Unknown order ID: ${id}`);
 const destination=new URL(`orders/${name}`,media);
 // Keep user-supplied HD bases. Never restore old pre-stamped P/M variants.
 try{await access(destination);}catch(error){if(error.code!=='ENOENT')throw error;await cp(new URL(`images/orders/${name}`,publicRoot),destination);}
 registry.orders[id]=`/media/orders/${name.replace(/\.png$/,'.webp')}`;
}
for(const sound of ['notification','combat','horse-gallop','raven_call','sword-cutting-and-killing']) await cp(new URL('sounds/'+sound+'.ogg',publicRoot),new URL('sounds/'+sound+'.ogg',media));
await writeFile(new URL('manifest.json',media),JSON.stringify(registry));
console.log('Exported selected unit figures, order tokens, crests and sounds. House portraits use the local HD WebP index.');
