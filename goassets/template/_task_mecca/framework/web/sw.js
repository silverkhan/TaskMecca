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
