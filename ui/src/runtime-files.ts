type Transport={rpc:(method:string,args?:unknown)=>Promise<any>};
const allowed=new Set('gif png jpg jpeg webp mp4 webm txt json md docx pptx epub csv ttf html svg pdf xlsx zip'.split(' '));
let saving=false;
export function installFiles(transport:Transport,request:(path:string,options?:RequestInit)=>Promise<any>){
 const downloads=Object.freeze({async save({filename,data,request:token}:{filename:string;data:string|Blob|ArrayBuffer|ArrayBufferView;request?:string}){
  if(token)throw {code:'request_unknown',message:'Export requests are not enabled'};
  if(typeof filename!=='string'||filename.length>512)throw {code:'bad_request',message:'Invalid filename'};
  filename=filename.replace(/[\x00-\x1f\x7f/\\]/g,'').replace(/\s+/g,' ').trim();while(new TextEncoder().encode(filename).length>240)filename=filename.slice(0,-1);
  if(!allowed.has(filename.split('.').at(-1)!.toLowerCase()))throw {code:'rejected_extension',message:'File extension is not allowed'};
  let blob:Blob;
  if(typeof data==='string')blob=new Blob([data]);else if(data instanceof Blob)blob=data;else if(data instanceof ArrayBuffer)blob=new Blob([structuredClone(data,{transfer:[data]})]);else if(ArrayBuffer.isView(data))blob=new Blob([new Uint8Array(data.buffer,data.byteOffset,data.byteLength).slice()]);else throw {code:'bad_request',message:'Invalid file data'};
  if(!blob.size)throw {code:'bad_request',message:'File is empty'};if(saving)throw {code:'rate_limited',message:'Another save is waiting'};saving=true;
  try{if(!confirm(`Save ${filename} (${blob.size.toLocaleString()} bytes)?`))throw {code:'declined',message:'Save declined'};const url=URL.createObjectURL(blob);const a=document.createElement('a');a.href=url;a.download=filename;a.click();setTimeout(()=>URL.revokeObjectURL(url),30000);return {status:'saved'}}finally{saving=false}
 }});
 const files=Object.freeze({
  list:(options:object={})=>transport.rpc('files.list',options),
  async read(path:string,options:{offset?:number;length?:number}={}){const params=new URLSearchParams({path,...Object.fromEntries(Object.entries(options).map(([k,v])=>[k,String(v)]))});const r=await request('/api/files?'+params) as Response;return {bytes:new Uint8Array(await r.arrayBuffer()),size:Number(r.headers.get('X-File-Size')),revision:r.headers.get('X-File-Revision')!}},
  async download(path:string){const r=await files.read(path);return downloads.save({filename:path.split('/').at(-1)!,data:new Blob([r.bytes])})},
  asSampleTools:async()=>[
   {name:'list_project_files',description:'Lists project files; returns relative paths and sizes.',inputSchema:{type:'object',properties:{path:{type:'string'}}},execute:(input:any)=>files.list({path:String(input.path??'')})},
   {name:'read_project_file',description:'Reads up to 12 KB of UTF-8 text from a project file.',inputSchema:{type:'object',properties:{path:{type:'string'}},required:['path']},execute:async(input:any)=>new TextDecoder().decode((await files.read(String(input.path),{length:12000})).bytes)},
  ],
 });return {files,downloads};
}
