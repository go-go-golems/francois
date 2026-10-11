type Transport={rpc:(method:string,args?:unknown)=>Promise<any>;connected:()=>boolean;subscribe:(callback:(e:any)=>void)=>()=>void};
export function installRooms(transport:Transport){
 const groups=new Map<string,ReturnType<typeof create>>();
 function create(name:string){
  let peers:any[]=Object.freeze([]) as unknown as any[];let presence:Record<string,unknown>={};let timer:ReturnType<typeof setTimeout>|undefined;let frame=0;
  const events=new Map<string,Set<(e:any)=>void>>();const listeners=new Set<(e:any)=>void>();const connections=new Set<(up:boolean)=>void>();const failures=new Set<(e:any)=>void>();let joined=true;
  const stop=transport.subscribe(e=>{
   if(e.type==='connection'){for(const f of connections)f(e.up);if(e.up&&joined){if(name)void transport.rpc('room.join',{room:name}).catch(report);if(Object.keys(presence).length)void transport.rpc('room.presence',{patch:presence}).catch(report)}}
   if(e.room!==name)return;
   if(e.type==='room_event')for(const f of events.get(e.topic)??[])f(Object.freeze(e));
   if(e.type==='room_peers'){
    const before=peers;const now=Date.now();peers=Object.freeze(e.peers.map((p:any)=>{const prev=before.find(x=>x.peer===p.peer);return Object.freeze({...p,presence:Object.freeze(p.presence),updatedAt:prev&&JSON.stringify(prev.presence)===JSON.stringify(p.presence)?prev.updatedAt:now})})) as unknown as any[];
    cancelAnimationFrame(frame);frame=requestAnimationFrame(()=>{const change={peers,joined:peers.filter(p=>!before.some(x=>x.peer===p.peer)),left:before.filter(p=>!peers.some(x=>x.peer===p.peer)),updated:peers.filter(p=>before.some(x=>x.peer===p.peer&&x.updatedAt!==p.updatedAt))};for(const f of listeners)f(change)});
   }
  });
  function report(e:any){for(const f of failures)f(e)}
  return Object.freeze({
   emit:(topic:string,data?:unknown)=>transport.connected()?transport.rpc('room.emit',{room:name,topic,data}):Promise.reject({code:'upstream_error',message:'Room disconnected'}),
   on:(topic:string,f:(e:any)=>void,onError?:(e:any)=>void)=>{if(!events.has(topic))events.set(topic,new Set());events.get(topic)!.add(f);if(onError)failures.add(onError);return()=>{events.get(topic)?.delete(f);if(onError)failures.delete(onError)}},
   presence:(patch:Record<string,unknown>)=>{for(const [key,value]of Object.entries(patch)){if(value===null)delete presence[key];else presence[key]=value};clearTimeout(timer);timer=setTimeout(()=>void transport.rpc('room.presence',{patch:presence}).catch(report),33);},
   peers:()=>peers,
   onPeers:(f:(e:any)=>void,onError?:(e:any)=>void)=>{listeners.add(f);if(onError)failures.add(onError);queueMicrotask(()=>{if(listeners.has(f))f({peers,joined:peers,left:[],updated:[]})});return()=>{listeners.delete(f);if(onError)failures.delete(onError)}},
   connected:()=>transport.connected(),onConnection:(f:(up:boolean)=>void)=>{connections.add(f);f(transport.connected());return()=>connections.delete(f)},
   join:async(room:string)=>{await transport.rpc('room.join',{room});if(!groups.has(room))groups.set(room,create(room));return groups.get(room)!},
   leave:async()=>{if(!name)return;joined=false;stop();clearTimeout(timer);cancelAnimationFrame(frame);groups.delete(name);await transport.rpc('room.leave',{room:name})},
   canSendToClaudeSession:async()=> 'off',sendToClaudeSession:async()=>{throw {code:'claude_unavailable',message:'No Claude conversation is attached'}},
  });
 }
 const lobby=create('');groups.set('',lobby);return lobby;
}
