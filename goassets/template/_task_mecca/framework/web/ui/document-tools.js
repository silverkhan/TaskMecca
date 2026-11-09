import { text } from './format.js';
let mermaidPromise,serial=0,lastTheme=document.documentElement.dataset.theme;
const diagrams=new Map();
function loadScript(url){return new Promise((resolve,reject)=>{if(window.mermaid)return resolve(window.mermaid);const script=document.createElement('script');script.src=url;script.async=true;script.onload=()=>window.mermaid?resolve(window.mermaid):reject(Error('Mermaid runtime unavailable'));script.onerror=()=>reject(Error('Mermaid runtime unavailable'));document.head.append(script);});}
async function mermaid(){return mermaidPromise??=loadScript('/vendor/mermaid.min.js').catch(()=>loadScript('https://cdnjs.cloudflare.com/ajax/libs/mermaid/11.10.0/mermaid.min.js')).catch(()=>loadScript('https://cdn.jsdelivr.net/npm/mermaid@11.10.0/dist/mermaid.min.js')).catch(e=>{mermaidPromise=null;throw e;});}
async function renderDiagram(pre,view,status){
  status.textContent=text('다이어그램을 그리는 중…','Rendering diagram…');
  try{const runtime=await mermaid();runtime.initialize({startOnLoad:false,securityLevel:'strict',theme:document.documentElement.dataset.theme==='dark'?'dark':'default'});const result=await runtime.render('tm-document-diagram-'+(++serial),pre.textContent);if(!pre.isConnected)return;view.innerHTML=result.svg;view.hidden=false;pre.hidden=true;status.textContent='';}
  catch(e){pre.hidden=false;view.hidden=true;status.textContent=text('다이어그램을 표시하지 못했습니다. 원문을 확인하거나 다시 시도하세요.','Could not render the diagram. Read the source or retry.');}
}
export function bindDocumentTools(root){
  for(const region of root.querySelectorAll('.management-sheet > .markdown,.document-section > .markdown')){
    region.tabIndex=0;region.setAttribute('role','region');region.setAttribute('aria-label',region.parentElement.querySelector('h2')?.textContent||text('문서 본문','Document content'));
  }
  for(const pre of root.querySelectorAll('.markdown pre,.raw-source pre')){
    if(pre.dataset.documentTools)continue;pre.dataset.documentTools='1';
    const source=pre.textContent,toolbar=document.createElement('div'),copy=document.createElement('button'),status=document.createElement('span');toolbar.className='document-tools';status.setAttribute('role','status');copy.type='button';copy.textContent=text('코드 복사','Copy code');copy.onclick=async()=>{try{await navigator.clipboard.writeText(source);status.textContent=text('복사했습니다','Copied');}catch{status.textContent=text('복사하지 못했습니다. 원문을 선택해 복사하세요.','Copy failed. Select and copy the source.');}};toolbar.append(copy);
    if(pre.dataset.codeLanguage==='mermaid'){
      const show=document.createElement('button'),view=document.createElement('div');show.type='button';show.textContent=text('다이어그램 보기 / 원문','Diagram / source');view.className='document-diagram';view.hidden=true;view.setAttribute('role','img');view.setAttribute('aria-label',text('원문에서 생성한 다이어그램','Diagram rendered from source'));show.onclick=()=>{if(!view.hidden){view.hidden=true;pre.hidden=false;}else void renderDiagram(pre,view,status);};toolbar.append(show);pre.after(view);diagrams.set(pre,{view,status});
    }
    toolbar.append(status);pre.before(toolbar);
  }
  for(const pre of diagrams.keys())if(!pre.isConnected)diagrams.delete(pre);
}
new MutationObserver(()=>{const theme=document.documentElement.dataset.theme;if(theme===lastTheme)return;lastTheme=theme;for(const [pre,{view,status}]of diagrams){if(!pre.isConnected){diagrams.delete(pre);continue;}if(!view.hidden)void renderDiagram(pre,view,status);}}).observe(document.documentElement,{attributes:true,attributeFilter:['data-theme']});
