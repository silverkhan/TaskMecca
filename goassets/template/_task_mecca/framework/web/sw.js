// Web Push arrives here even when all Task Mecca tabs are closed.
// The service worker never decides delivery eligibility: the server-side
// canonical-event ledger has already granted the unique claim.
self.addEventListener('push', event=>{
  if(!event.data)return;
  let payload;
  try{payload=event.data.json();}catch(_){return;}
  if(!payload||typeof payload!=='object')return;
  const title=String(payload.title||'Task Mecca').slice(0,180);
  const body=String(payload.body||'').slice(0,600);
  const tag=String(payload.tag||'task-mecca-push').slice(0,220);
  const path=typeof payload.url==='string'&&payload.url.startsWith('/')&&!payload.url.startsWith('//')
    ?payload.url:'/?view=notifications';
  event.waitUntil(self.registration.showNotification(title,{
    body,tag,data:{url:path},renotify:false
  }));
});

self.addEventListener('notificationclick',event=>{
  event.notification.close();
  const target=event.notification?.data?.url||'/';
  event.waitUntil((async()=>{
    const windows=await clients.matchAll({type:'window',includeUncontrolled:true});
    if(windows.length){
      const client=windows[0];
      if('navigate' in client) await client.navigate(target);
      return client.focus();
    }
    return clients.openWindow(target);
  })());
});
