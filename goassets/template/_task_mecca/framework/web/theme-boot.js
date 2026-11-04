// Apply the saved appearance before style.css and the main application load.
// This blocking, same-origin script is allowed by the existing CSP.
(() => {
  const root=document.documentElement;
  let storedTheme='',storedPalette='',storedBackground='';
  try {
    storedTheme=localStorage.getItem('task-mecca-theme')||'';
    storedPalette=localStorage.getItem('task-mecca-palette')||'';
    storedBackground=localStorage.getItem('task-mecca-background')||'';
  }catch(_){}
  const preference=['dark','light','system'].includes(storedTheme)?storedTheme:'dark';
  const palette=['mecca','slate','glass'].includes(storedPalette)?storedPalette:'mecca';
  const backdrop=['cozy','ambient','none'].includes(storedBackground)?storedBackground:'cozy';
  const systemDark=window.matchMedia?.('(prefers-color-scheme: dark)')?.matches ?? false;
  const effective=preference==='system'?(systemDark?'dark':'light'):preference;
  const background=palette==='glass'?(backdrop==='cozy'?(effective==='dark'?'#211b1b':'#e9dfce'):(effective==='dark'?'#0b1020':'#edf2fa'))
    :palette==='mecca'?(effective==='dark'?'#0a0e1b':'#f7f3f8')
    :(effective==='dark'?'#111318':'#f7f8fa');
  root.dataset.themePreference=preference;
  root.dataset.theme=effective;
  root.dataset.palette=palette;
  root.dataset.background=backdrop;
  // Cover the time before the external stylesheet becomes available.
  root.style.colorScheme=effective;
  root.style.backgroundColor=background;
  const meta=document.querySelector('meta[name="theme-color"]');
  if(meta)meta.setAttribute('content',background);
})();
