import React,{useEffect,useRef,useState} from 'react';
import {createRoot} from 'react-dom/client';
import {configureStore,createSlice} from '@reduxjs/toolkit';
import {createApi,fetchBaseQuery} from '@reduxjs/toolkit/query/react';
import {Provider,useDispatch,useSelector} from 'react-redux';
type Artifact={id:string;title:string;version:number;url:string;manifest:Record<string,unknown>};
type Model={provider:string;id:string;name:string;tier?:string};
type Comment={threadId:string;id:string;text:string;anchor:{text?:string};resolved:boolean;createdAt:string};
const api=createApi({reducerPath:'api',baseQuery:fetchBaseQuery({baseUrl:'/api'}),tagTypes:['Artifacts','Comments'],endpoints:b=>({
 artifacts:b.query<Artifact[],void>({query:()=>'/artifacts',providesTags:['Artifacts']}),models:b.query<Model[],void>({query:()=>'/models'}),
 import:b.mutation<Artifact,object>({query:body=>({url:'/artifacts',method:'POST',body}),invalidatesTags:['Artifacts']}),
 comments:b.query<Comment[],void>({query:()=>'/threads',providesTags:['Comments']}),
 write:b.mutation<unknown,{view:string;method:string;args:object}>({query:({view,method,args})=>({url:`/views/${view}/rpc`,method:'POST',headers:{'X-Francois-View':view},body:{method,args}}),invalidatesTags:['Comments']}),
})});
const ui=createSlice({name:'ui',initialState:{importing:false},reducers:{toggle:s=>{s.importing=!s.importing}}});
const store=configureStore({reducer:{[api.reducerPath]:api.reducer,ui:ui.reducer},middleware:g=>g().concat(api.middleware)});
function Library(){
 const {data:artifacts=[],isLoading,error}=api.useArtifactsQuery();const {data:models=[]}=api.useModelsQuery();const [importFile,{isLoading:importing,error:importError}]=api.useImportMutation();
 const open=useSelector((s:ReturnType<typeof store.getState>)=>s.ui.importing);const dispatch=useDispatch();
 const [file,setFile]=useState<File>();const [id,setId]=useState('');const [title,setTitle]=useState('');const [manifest,setManifest]=useState('{"sample":{}}');const [localError,setLocalError]=useState('');
 const submit=async(e:React.FormEvent)=>{e.preventDefault();try{if(!file)throw new Error('Choose an HTML file');setLocalError('');await importFile({id,title:title||id,html:await file.text(),manifest:JSON.parse(manifest)}).unwrap();dispatch(ui.actions.toggle())}catch(e){setLocalError(e instanceof Error?e.message:'Could not import this file')}};
 return <><header><strong>Francois</strong><span>Artifact library</span></header><main><div className="heading"><div><h1>Your artifacts</h1><p>Open an application with your local model connection.</p></div><button className="primary" onClick={()=>dispatch(ui.actions.toggle())}>{open?'Close':'Import HTML'}</button></div>
 {open&&<form className="panel form" onSubmit={submit}><h2>Import an artifact</h2><label>HTML file<input type="file" accept=".html,.htm,text/html" onChange={e=>{const f=e.target.files?.[0];setFile(f);if(f){setId(f.name.replace(/\.html?$/i,'').toLowerCase().replace(/[^a-z0-9]+/g,'-').replace(/^-|-$/g,''));setTitle(f.name.replace(/\.html?$/i,''))}}}/></label><label>Artifact ID<input value={id} onChange={e=>setId(e.target.value)} required pattern="[a-z0-9][a-z0-9_-]*"/></label><label>Title<input value={title} onChange={e=>setTitle(e.target.value)}/></label><label>Capabilities<textarea value={manifest} onChange={e=>setManifest(e.target.value)} spellCheck={false}/></label>{(localError||importError)&&<p className="error">{localError||'Import failed'}</p>}<button className="primary" disabled={importing}>{importing?'Importing…':'Import artifact'}</button></form>}
 {error&&<p className="error">Could not load artifacts.</p>}{isLoading?<p>Loading…</p>:artifacts.length?<div className="grid">{artifacts.map(a=><article className="panel" key={a.id}><div className="row"><h2 className="grow">{a.title}</h2><span className="pill">v{a.version}</span></div><div className="meta">{Object.keys(a.manifest).join(' · ')||'HTML'}</div><div className="row"><a className="button primary" href={a.url}>Open artifact</a>{a.manifest.comments!==undefined&&<a className="button" href={a.url+'/shell'}>Comments</a>}</div></article>)}</div>:<div className="panel empty"><h2>No artifacts yet</h2><p>Import a saved HTML page to get started.</p></div>}
 <section className="models"><h2>Model connection</h2><p>Your model access is managed by Pi.</p>{models.map((m,i)=><div className="model" key={i}><span className="grow">{m.name}</span>{m.tier&&<span className="pill">{m.tier}</span>}<code>{m.provider}/{m.id}</code></div>)}</section></main></>;
}
function Shell(){
 const {data:artifacts=[]}=api.useArtifactsQuery();const {data:comments=[]}=api.useCommentsQuery(undefined,{pollingInterval:5000});const [write]=api.useWriteMutation();const [composer,setComposer]=useState<{anchor:object;view:string}>();const [text,setText]=useState('');const [error,setError]=useState('');const frame=useRef<HTMLIFrameElement>(null);
 useEffect(()=>{const receive=(e:MessageEvent)=>{if(e.origin!==location.origin||e.source!==frame.current?.contentWindow||e.data?.type!=='francois-compose')return;setComposer({anchor:e.data.anchor,view:e.data.viewId})};addEventListener('message',receive);return()=>removeEventListener('message',receive)},[]);
 const post=async()=>{if(!composer)return;try{await write({view:composer.view,method:'comments.create',args:{anchor:composer.anchor,text}}).unwrap();setComposer(undefined);setText('');setError('')}catch{setError('Could not save comment')}};
 const threads=[...new Set(comments.map(c=>c.threadId))];
 return <><header><a href="/">←</a><strong>{artifacts[0]?.title||'Artifact'}</strong><span className="grow">Comments</span><a href="/">Open full page</a></header><div className="shell"><iframe ref={frame} title="Artifact" src="/"/><aside className="comments"><h2>Comments</h2>{composer?<div className="form"><textarea aria-label="Comment" value={text} onChange={e=>setText(e.target.value)}/><div className="row"><button className="primary" onClick={post} disabled={!text.trim()}>Post</button><button onClick={()=>setComposer(undefined)}>Cancel</button></div></div>:<p className="notice">Use the artifact’s Comment button to start a thread.</p>}{error&&<p className="error">{error}</p>}{threads.map(thread=>{const entries=comments.filter(c=>c.threadId===thread);return <section className="thread" key={thread}><blockquote>{entries[0].anchor.text||'Artifact'}</blockquote>{entries.map(c=><div key={c.id}><p>{c.text}</p><time>{new Date(c.createdAt).toLocaleString()}</time></div>)}</section>})}</aside></div></>;
}
createRoot(document.getElementById('root')!).render(<Provider store={store}>{location.pathname==='/shell'?<Shell/>:<Library/>}</Provider>);
