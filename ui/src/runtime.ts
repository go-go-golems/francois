import {normalizeInput,parseJSON} from './sample-contract';
import { installDB } from './runtime-db';
import { installRooms } from './runtime-rooms';
import { installFiles } from './runtime-files';

type Failure={code:string;message:string;text?:string};
type Wire=Record<string,any>;
const boot=JSON.parse(document.getElementById('francois-view')!.textContent!);
const error=(code:string,message:string,text?:string):Failure=>({code,message,...(text?{text}:{})});
const listeners=new Set<(event:Wire)=>void>();
let view='';let up=false;let closed=false;
let readyResolve:()=>void;let readyPromise=new Promise<void>(r=>readyResolve=r);
const active=new Map<string,{event:(e:Wire)=>void;fail:(e:Failure)=>void}>();
const random=()=>crypto.randomUUID();
export async function request(path:string,options:RequestInit={}):Promise<any>{
 const response=await fetch(path,options);
 if(!response.ok){let failure;try{failure=(await response.json()).error}catch{failure=error('upstream_error','Host request failed')};throw failure;}
 return response.headers.get('content-type')?.includes('application/json')?response.json():response;
}
export const transport={
 ready:()=>readyPromise,
 connected:()=>up,
 subscribe:(fn:(event:Wire)=>void)=>{listeners.add(fn);return()=>listeners.delete(fn)},
 async post(path:string,data:unknown){await readyPromise;return request(`/api/views/${view}/${path}`,{method:'POST',headers:{'Content-Type':'application/json','X-Francois-View':view},body:JSON.stringify(data)})},
 async rpc(method:string,args:unknown={}){return this.post('rpc',{method,args})},
 view:()=>view,
};
async function connect(){
 while(!closed){
  try{
   const created=await request('/api/views',{method:'POST',headers:{'Content-Type':'application/json'},body:JSON.stringify({version:boot.version})});view=created.viewId;
   const response=await request(`/api/views/${view}/events`,{headers:{'X-Francois-View':view}}) as Response;
   const reader=response.body!.getReader();const decoder=new TextDecoder();let buffer='';
   while(!closed){const chunk=await reader.read();if(chunk.done)break;buffer+=decoder.decode(chunk.value,{stream:true});let pos;
    while((pos=buffer.indexOf('\n\n'))>=0){const block=buffer.slice(0,pos);buffer=buffer.slice(pos+2);if(!block.startsWith('data: '))continue;const event=JSON.parse(block.slice(6));
     if(event.type==='ready'){up=true;readyResolve!();for(const f of listeners)f({type:'connection',up:true});}
     else if(event.type==='reload'){location.reload();return;}
     else if(event.callId){active.get(event.callId)?.event(event);}
     else{for(const f of listeners){try{f(event)}catch(e){console.error('Francois listener failed',e)}}}
    }
   }
  }catch(e){if(!closed)console.warn('Francois connection interrupted');}
  up=false;for(const call of active.values())call.fail(error('upstream_error','Connection interrupted'));active.clear();
  readyPromise=new Promise<void>(r=>readyResolve=r);for(const f of listeners)f({type:'connection',up:false});
  if(!closed)await new Promise(r=>setTimeout(r,1000));
 }
}
void connect();addEventListener('pagehide',()=>{closed=true});

type Tool={name:string;description:string;inputSchema?:object;execute:(input:Record<string,unknown>,ctx:{signal:AbortSignal})=>unknown};
type Options={modelTier?:'quick'|'default'|'complex';model?:string;effort?:'low'|'medium'|'high';cache?:boolean|{gcTime?:number;refresh?:boolean};signal?:AbortSignal;onText?:(e:{text:string;delta:string})=>void;tools?:Tool[];images?:unknown};
type Result={text:string;finalText?:string;truncated:boolean;modelTierApplied:string;modelApplied?:string};
const plain=(v:unknown):v is Record<string,unknown>=>v!==null&&typeof v==='object'&&(Object.getPrototypeOf(v)===Object.prototype||Object.getPrototypeOf(v)===null);
const bytes=(s:string)=>new TextEncoder().encode(s).length;

function capture(input:unknown,options:Options){
 if(!plain(options))throw error('invalid_request','Options must be a plain object');
 const normalized=normalizeInput(input);
 if(options.images!==undefined)throw error('images_unavailable','Image sampling is not enabled');
 if(options.signal!==undefined&&!(options.signal instanceof AbortSignal))throw error('invalid_request','Invalid abort signal');
 if(options.onText!==undefined&&typeof options.onText!=='function')throw error('invalid_request','Invalid text callback');
 if(options.modelTier!==undefined&&!['quick','default','complex'].includes(options.modelTier))throw error('invalid_request','Invalid model tier');
 if(options.effort!==undefined&&!['low','medium','high'].includes(options.effort))throw error('invalid_request','Invalid effort');
 if(options.model!==undefined&&typeof options.model!=='string')throw error('invalid_request','Invalid model');
 const tools=options.tools??[];if(!Array.isArray(tools)||tools.length>16)throw error('invalid_request','Invalid tools');
 if(tools.length&&options.cache!==undefined&&options.cache!==false)throw error('invalid_request','Tools cannot be cached');
 const names=new Set<string>();const descriptors=[];const callbacks=new Map<string,Tool['execute']>();
 for(const t of tools){if(!plain(t)||!/^[-A-Za-z0-9_]{1,128}$/.test(t.name)||names.has(t.name)||typeof t.description!=='string'||bytes(t.description)>1024||typeof t.execute!=='function')throw error('invalid_request','Invalid tool');names.add(t.name);if(t.inputSchema!==undefined&&(!plain(t.inputSchema)||t.inputSchema.type!=='object'||bytes(JSON.stringify(t.inputSchema))>4096))throw error('invalid_request','Invalid tool schema');descriptors.push(JSON.parse(JSON.stringify({name:t.name,description:t.description,inputSchema:t.inputSchema})));callbacks.set(t.name,t.execute);}
 const policy=options.cache??true;if(typeof policy!=='boolean'&&!plain(policy))throw error('invalid_request','Invalid cache policy');
 const ttl=typeof policy==='object'?policy.gcTime??300000:300000;if(typeof ttl!=='number'||ttl<0||!Number.isFinite(ttl))throw error('invalid_request','Invalid cache lifetime');
 return {input:normalized,options:{modelTier:options.modelTier??'default',model:options.model,effort:options.effort},descriptors,callbacks,signal:options.signal,onText:options.onText,ttl:Math.min(ttl,86400000),cache:!tools.length&&policy!==false,refresh:typeof policy==='object'&&policy.refresh===true};
}
type Captured=ReturnType<typeof capture>;
function safeText(callback:Options['onText'],event:{text:string;delta:string}){try{callback?.(event)}catch(e){console.error('sample onText failed',e)}}
function rawCall(c:Captured,signal:AbortSignal,onText:Options['onText']):Promise<Result>{
 return new Promise(async(resolve,reject)=>{
  let text='';const callId=random();const children=new Set<AbortController>();let done=false;
  const settle=(fn:()=>void)=>{if(done)return;done=true;signal.removeEventListener('abort',abort);for(const child of children)child.abort();active.delete(callId);fn();};
  const abort=()=>{settle(()=>reject(error('cancelled','Stopped',text)));if(up)void transport.post(`calls/${callId}/cancel`,{}).catch(()=>{});};
  signal.addEventListener('abort',abort,{once:true});if(signal.aborted){abort();return;}
  active.set(callId,{fail:e=>settle(()=>reject({...e,...(text?{text}:{})})),event:e=>{
   if(e.type==='text'){text=e.text;safeText(onText,{text,delta:e.delta});}
   if(e.type==='result'){settle(()=>resolve(e.result));}
   if(e.type==='error'){settle(()=>reject({...e.error,...(e.error.code==='refused'?{}:text?{text}:{} )}));}
   if(e.type==='tool')void(async()=>{
    const ctl=new AbortController();children.add(ctl);const timer=setTimeout(()=>ctl.abort(),150000);const callback=c.callbacks.get(e.name);
    let value:unknown,isError=false;
    try{if(!callback)throw new Error('Unknown tool');const aborted=new Promise<never>((_,no)=>ctl.signal.addEventListener('abort',()=>no(new Error('Tool stopped')),{once:true}));value=await Promise.race([callback(e.input,{signal:ctl.signal}),aborted]);if(typeof value!=='string'&&!plain(value)&&!Array.isArray(value)&&typeof value!=='number'&&typeof value!=='boolean'&&value!==null)throw new Error('Tool result must be string or JSON');value=JSON.parse(JSON.stringify(value));if(bytes(JSON.stringify(value))>32768)throw new Error('Tool result exceeds 32 KiB');}
    catch(e){isError=true;value='Error: '+(e instanceof Error?e.message:'Tool execution failed');}
    finally{clearTimeout(timer);ctl.abort();children.delete(ctl);}
    if(!done)await transport.post(`calls/${callId}/tools/${e.toolCallId}/result`,{value,isError}).catch(()=>{});
   })();
  }});
  try{await transport.ready();if(done||signal.aborted)return;await transport.post('calls',{callId,input:c.input,options:c.options,tools:c.descriptors});}catch(e){settle(()=>reject(e));}
 });
}
const cache=new Map<string,{at:number;ttl:number;value:any;text:string}>();
const flights=new Map<string,{ctl:AbortController;promise:Promise<any>;subscribers:Set<{signal?:AbortSignal;onText?:Options['onText']}>}>();
function invoke(input:unknown,options:Options={},json=false):Promise<any>{
 let c:Captured|undefined;let invalid:unknown;
 try{c=capture(input,options)}catch(e){invalid=e}
 return Promise.resolve().then(async()=>{
  if(invalid)throw invalid;const captured=c!;if(captured.signal?.aborted)throw error('cancelled','Stopped');
  const key=JSON.stringify([captured.input,captured.options,json]);
  const prior=cache.get(key);
  if(captured.cache&&!captured.refresh&&prior&&Date.now()-prior.at<Math.min(prior.ttl,captured.ttl)){safeText(captured.onText,{text:prior.text,delta:prior.text});return structuredClone(prior.value);}
  const sharedKey=captured.cache?key:random();let flight=flights.get(sharedKey);
  if(!flight){const ctl=new AbortController();const subscribers=new Set<{signal?:AbortSignal;onText?:Options['onText']}>();
   const promise=rawCall(captured,ctl.signal,e=>{for(const s of subscribers)safeText(s.onText,e)}).then(result=>{
    const value=json?parseJSON(result.finalText??result.text,result.truncated):{text:result.text,truncated:result.truncated,modelTierApplied:result.modelTierApplied,...(result.modelApplied?{modelApplied:result.modelApplied}:{})};
    if(captured.cache){cache.set(key,{at:Date.now(),ttl:captured.ttl,value,text:result.text});if(cache.size>100)cache.delete(cache.keys().next().value!);}return value;
   }).finally(()=>flights.delete(sharedKey));flight={ctl,promise,subscribers};flights.set(sharedKey,flight);
  }
  const current=flight;const subscriber={signal:captured.signal,onText:captured.onText};current.subscribers.add(subscriber);
  return new Promise((resolve,reject)=>{
   const abort=()=>{current.subscribers.delete(subscriber);reject(error('cancelled','Stopped'));if(!current.subscribers.size)current.ctl.abort();};
   captured.signal?.addEventListener('abort',abort,{once:true});
   current.promise.then(v=>{if(!captured.signal?.aborted)resolve(structuredClone(v))},reject).finally(()=>{captured.signal?.removeEventListener('abort',abort);current.subscribers.delete(subscriber)});
  });
 });
}
const sample=Object.assign((input:unknown,options?:Options)=>invoke(input,options),{json:(input:unknown,options?:Options)=>invoke(input,options,true),limits:async()=>Object.freeze({maxPromptBytes:262144,tools:Object.freeze({maxCount:16})})});Object.freeze(sample);
const manifest=boot.manifest as Record<string,any>;
const profile=(uid:string)=>Object.freeze({id:uid,name:manifest.user?.scopes?.includes('profile')&&uid===boot.owner?'Local owner':'',avatarUrl:'data:image/svg+xml,%3Csvg xmlns="http://www.w3.org/2000/svg" width="32" height="32"%3E%3Crect width="32" height="32" rx="16" fill="%23eaf1fe"/%3E%3C/svg%3E',color:'#2563eb',email:null,isMe:uid===boot.owner,guest:false});
const user=Object.freeze({id:async()=>boot.owner,name:async()=>profile(boot.owner).name,email:async()=>null,avatarUrl:async()=>profile(boot.owner).avatarUrl,me:async()=>Object.freeze({...profile(boot.owner),isOwner:true,canEdit:true}),isOwner:async()=>true,canEdit:async()=>true,can:async(name:string)=>['data.write','files.write','assets.write','people.add'].includes(name)&&name!=='people.add',profiles:async(ids:string[])=>Object.fromEntries(ids.map(uid=>[uid,profile(uid)])),search:async(q:string)=>manifest.user?.scopes?.includes('profile')&&'local owner'.includes(String(q).toLowerCase())?[profile(boot.owner)]:[]});
const comments=Object.freeze({
 anchorFor:(element:Element)=>({id:element.id||null,text:element.textContent?.slice(0,500)??'',tag:element.tagName.toLowerCase()}),
 async openComposer({element,range}:{element?:Element;range?:Range}){if(parent===window)return {opened:false};await transport.ready();const anchor=element?comments.anchorFor(element):{text:range?.toString().slice(0,500)??''};parent.postMessage({type:'francois-compose',anchor,viewId:view},location.origin);return {opened:true};},
 ...(manifest.comments?.composer_only?{}:{create:({anchor,text}:any)=>transport.rpc('comments.create',{anchor,text}),reply:(threadId:string,text:string)=>transport.rpc('comments.reply',{threadId,text}),resolve:(threadId:string,resolved:boolean)=>transport.rpc('comments.resolve',{threadId,resolved}),delete:(threadId:string)=>transport.rpc('comments.delete',{threadId}),canSendToClaude:async()=> 'no_session',sendToClaude:async()=>{throw error('claude_unavailable','No Claude conversation is attached')}}),
});
const capabilities:Record<string,unknown>={sample,user,comments,db:installDB(transport),room:installRooms(transport),...installFiles(transport,request),artifact:Object.freeze({publish:(html:unknown)=>typeof html==='string'?transport.rpc('artifact.publish',{html}):Promise.reject(error('capability_removed','Files-form publishing is not enabled'))}),assets:Object.freeze({async upload(blob:Blob,options:{type?:string}={}){return request('/api/assets',{method:'POST',headers:{'Content-Type':options.type??blob.type},body:blob})},list:()=>transport.rpc('assets.list'),delete:(key:string)=>transport.rpc('assets.delete',{id:key.split('/').at(-1)})})};
Object.defineProperty(window,'claude',{value:Object.freeze({use:async(name:string)=>manifest[name]!==undefined?capabilities[name]??null:null}),writable:false,configurable:false});
