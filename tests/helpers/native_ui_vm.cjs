// Execute the real native modules in a small DOM VM. Imports resolve to the
// actual production module files, not extracted legacy app.js or fake helpers.
const fs=require('node:fs'),path=require('node:path'),vm=require('node:vm');
function session(globals={},root=path.resolve('goassets/template/_task_mecca/framework/web/ui')){
 const context=vm.createContext({URL,URLSearchParams,AbortController,Intl,Date,console,setTimeout,clearTimeout,queueMicrotask,...globals});const cache=new Map();
 function load(relative,observe=[]){const file=path.resolve(root,relative);if(cache.has(file))return cache.get(file);let source=fs.readFileSync(file,'utf8'),exports=[];
  source=source.replace(/^\s*import\s+([^;]+?)\s+from\s+['"]([^'"]+)['"];?/gm,(_,names,spec)=>{const resolved=path.resolve(path.dirname(file),spec);let binding;if(names.trim().startsWith('{'))binding=names.trim().replace(/\bas\b/g,':');else if(names.trim().startsWith('* as '))binding=names.trim().slice(5);else throw Error('Unsupported native import: '+names);return 'const '+binding+'=__import('+JSON.stringify(resolved)+');';});
  source=source.replace(/export\s+((?:async\s+)?function|const|let|var)\s+(\w+)/g,(_,kind,name)=>{exports.push([name,name]);return kind+' '+name});
  source=source.replace(/export\s*\{([^}]+)\};?/g,(_,list)=>{for(const value of list.split(',').filter(Boolean)){const [name,alias]=value.trim().split(/\s+as\s+/);exports.push([name,alias||name])}return ''});
  for(const name of observe)if(!exports.some(([n])=>n===name))exports.push([name,name]);
  const run=vm.runInContext('(function(__import){'+source+'\nreturn {'+exports.map(([name,alias])=>'get '+alias+'(){return '+name+'}').join(',')+'};})',context,{filename:file});const result=run(spec=>load(path.relative(root,spec)));cache.set(file,result);return result;
 }
 return {context,load};
}
module.exports={session};
